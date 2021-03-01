# Streaming mode for short audio recognition (speech-to-text)

Set the `FOLDER_ID`, `IAM_TOKEN` variables and specify the path to the audio file in the arguments:

```bash
$ export FOLDER_ID=b1gvmob95yysaplct532
$ export IAM_TOKEN=CggaATEVAgA...
$ go run . "./speech.pcm"

Start chunk: alternative: прив
Is final: false
Start chunk: alternative: привет ми
Is final: false
Start chunk: alternative: Привет мир
Is final: true
```
