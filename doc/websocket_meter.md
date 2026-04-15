### Load Testing

```
root@hackers365-System-Product-Name:~# docker run -itd --name websocket_meter docker.jsdelivr.fyi/hackers365/xiaozhi_websocket_client                      
87311584e5fef592f32e0b7d7062d9053e956d5e0d50edb220370ff37d2293ac
root@hackers365-System-Product-Name:~# 
root@hackers365-System-Product-Name:~# docker exec -it websocket_meter /bin/bash                                                      
root@87311584e5fe:/workspace# 
root@87311584e5fe:/workspace# ./ws_multi  -h
Usage of ./ws_multi:
  -count int
        Number of clients (default 10)
  -device string
        Device ID
  -server string
        Server address (default "ws://localhost:8989/xiaozhi/v1/")
  -text string
        Chat content, multiple sentences separated by commas will be sent sequentially (default "Hello")
root@87311584e5fe:/workspace# ./ws_multi -count 1 -server wss://joeyzhou.chat/ws/xiaozhi/v1/ -text "Hello,What are you doing,Let's go out and play" 
Running Xiaozhi client
Server: wss://joeyzhou.chat/ws/xiaozhi/v1/
Number of clients: 1
Content to send: Hello,What are you doing,Let's go out and play
2025-05-27 09:54:51.095 [info] [audio_utils.go:199] TTS cloud first frame time: 532 ms
2025-05-27 09:54:51.098 [info] [audio_utils.go:269] TTS cloud->first frame decode completed time: 535 ms
2025-05-27 09:54:51.401 [info] [cosyvoice.go:306] TTS time: from input to getting MP3 data end time: 838 ms
2025-05-27 09:54:51.748 [info] [audio_utils.go:199] TTS cloud first frame time: 344 ms
2025-05-27 09:54:51.752 [info] [audio_utils.go:269] TTS cloud->first frame decode completed time: 347 ms
2025-05-27 09:54:51.901 [info] [cosyvoice.go:306] TTS time: from input to getting MP3 data end time: 497 ms
2025-05-27 09:54:52.292 [info] [audio_utils.go:199] TTS cloud first frame time: 387 ms
2025-05-27 09:54:52.296 [info] [audio_utils.go:269] TTS cloud->first frame decode completed time: 391 ms
2025-05-27 09:54:52.628 [info] [cosyvoice.go:306] TTS time: from input to getting MP3 data end time: 723 ms
0 Client started running
0 Client connected to server: wss://joeyzhou.chat/ws/xiaozhi/v1/
Received message: {Type:hello Text: State: SessionID:cafd2800-1979-06d5-19cf-b8bf53bb55dc Transport:websocket AudioFormat:<nil>}
Sending Opus frame: 20
Sending Opus frame: 50
Sending Opus frame: 59
```

#### Overall Description
    1. The program will call the TTS interface to generate audio data based on user input text, and send it to the server sequentially
    2. Time statistics start from type: listen, state: stop until receiving the first frame of audio data from the server

#### Parameter Description:
    -count: Number of concurrent connections
    -device: By default, deviceId is randomly generated. If using this parameter to specify a device, -count must be 1
    -server: WebSocket server address
    -text: Content to send, separated by "," and sent in a loop

#### Output Description
    You can redirect output to a log file, then tail -f xx.log | grep 'Average response time'
