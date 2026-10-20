# Test audio

- `speech-*.wav` and `speech-*.flac` were generated with macOS `say` and `afconvert`. Each FLAC file holds exactly the same samples as the WAV file with the same name.
- `speech-aac-mono.m4b`, `speech-aac-stereo.m4a` (AAC-LC) and `speech-heaac-stereo.m4a` (HE-AAC) were made from the WAV files with macOS `afconvert`.
- `speech-chapters.m4b` is `speech-aac-mono.m4b` with three chapters added by a short script: *Opening* at 0 s, *The Middle* at 0.25 s and *Ending* at 0.5 s. They are stored both as a QuickTime chapter track and as a Nero `chpl` box, and macOS's AVFoundation reads them back.
- `hello-mono.ogg` is [En-us-hello.ogg](https://commons.wikimedia.org/wiki/File:En-us-hello.ogg) from Wikimedia Commons, recorded by Dvortygirl and released into the public domain.
