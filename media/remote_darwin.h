enum {
    remotePlay,
    remotePause,
    remoteToggle,
    remoteNext,
    remotePrevious,
    remoteSkipForward,
    remoteSkipBackward,
    remoteSeek,
};

void startRemote(double interval);
void stopRemote(void);
void setRemoteTrack(const char *title, const char *artist, const void *cover, int coverLength, double duration);
void setRemoteStatus(int playing, double position, double rate);
char *remoteTitle(void);
int remoteIsPlaying(void);
double remoteElapsed(void);
int remoteHasArtwork(void);
