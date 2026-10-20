#import <AppKit/AppKit.h>
#import <MediaPlayer/MediaPlayer.h>

#include <stdlib.h>
#include <string.h>

#include "remote_darwin.h"

extern void mediaCommand(int command, double position);

static NSDictionary *nowPlaying;

static void handle(MPRemoteCommand *command, int code) {
    command.enabled = YES;
    [command addTargetWithHandler:^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *event) {
        mediaCommand(code, 0);
        return MPRemoteCommandHandlerStatusSuccess;
    }];
}

static void publish(NSDictionary *info) {
    @synchronized ([MPNowPlayingInfoCenter class]) {
        nowPlaying = info;
        [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo = info;
    }
}

static NSMutableDictionary *current(void) {
    @synchronized ([MPNowPlayingInfoCenter class]) {
        return nowPlaying == nil ? [NSMutableDictionary dictionary] : [nowPlaying mutableCopy];
    }
}

void startRemote(double interval) {
    MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];

    handle(center.playCommand, remotePlay);
    handle(center.pauseCommand, remotePause);
    handle(center.togglePlayPauseCommand, remoteToggle);
    handle(center.nextTrackCommand, remoteNext);
    handle(center.previousTrackCommand, remotePrevious);

    center.skipForwardCommand.preferredIntervals = @[@(interval)];
    center.skipBackwardCommand.preferredIntervals = @[@(interval)];
    handle(center.skipForwardCommand, remoteSkipForward);
    handle(center.skipBackwardCommand, remoteSkipBackward);

    center.changePlaybackPositionCommand.enabled = YES;
    [center.changePlaybackPositionCommand addTargetWithHandler:^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *event) {
        mediaCommand(remoteSeek, ((MPChangePlaybackPositionCommandEvent *)event).positionTime);
        return MPRemoteCommandHandlerStatusSuccess;
    }];
}

void stopRemote(void) {
    MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];
    for (MPRemoteCommand *command in @[
        center.playCommand, center.pauseCommand, center.togglePlayPauseCommand,
        center.nextTrackCommand, center.previousTrackCommand,
        center.skipForwardCommand, center.skipBackwardCommand,
        center.changePlaybackPositionCommand,
    ]) {
        [command removeTarget:nil];
        command.enabled = NO;
    }

    publish(nil);
    [MPNowPlayingInfoCenter defaultCenter].playbackState = MPNowPlayingPlaybackStateStopped;
}

void setRemoteTrack(const char *title, const char *artist, const void *cover, int coverLength, double duration) {
    @autoreleasepool {
        NSMutableDictionary *previous = current();
        NSMutableDictionary *info = [NSMutableDictionary dictionary];

        info[MPMediaItemPropertyTitle] = [NSString stringWithUTF8String:title];
        if (strlen(artist) > 0) {
            info[MPMediaItemPropertyArtist] = [NSString stringWithUTF8String:artist];
        }
        info[MPMediaItemPropertyPlaybackDuration] = @(duration);
        info[MPNowPlayingInfoPropertyMediaType] = @(MPNowPlayingInfoMediaTypeAudio);
        info[MPNowPlayingInfoPropertyElapsedPlaybackTime] = previous[MPNowPlayingInfoPropertyElapsedPlaybackTime];
        info[MPNowPlayingInfoPropertyPlaybackRate] = previous[MPNowPlayingInfoPropertyPlaybackRate];

        if (coverLength > 0) {
            NSImage *image = [[NSImage alloc] initWithData:[NSData dataWithBytes:cover length:coverLength]];
            if (image != nil) {
                info[MPMediaItemPropertyArtwork] = [[MPMediaItemArtwork alloc]
                    initWithBoundsSize:image.size
                        requestHandler:^NSImage *(CGSize size) {
                            return image;
                        }];
            }
        }

        publish(info);
    }
}

void setRemoteStatus(int playing, double position, double rate) {
    @autoreleasepool {
        NSMutableDictionary *info = current();
        info[MPNowPlayingInfoPropertyElapsedPlaybackTime] = @(position);
        info[MPNowPlayingInfoPropertyPlaybackRate] = @(playing ? rate : 0.0);
        info[MPNowPlayingInfoPropertyDefaultPlaybackRate] = @(1.0);

        publish(info);
        [MPNowPlayingInfoCenter defaultCenter].playbackState =
            playing ? MPNowPlayingPlaybackStatePlaying : MPNowPlayingPlaybackStatePaused;
    }
}

char *remoteTitle(void) {
    NSString *title = [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo[MPMediaItemPropertyTitle];
    return strdup(title == nil ? "" : title.UTF8String);
}

int remoteIsPlaying(void) {
    return [MPNowPlayingInfoCenter defaultCenter].playbackState == MPNowPlayingPlaybackStatePlaying;
}

double remoteElapsed(void) {
    return [[MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo[MPNowPlayingInfoPropertyElapsedPlaybackTime] doubleValue];
}

int remoteHasArtwork(void) {
    return [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo[MPMediaItemPropertyArtwork] != nil;
}
