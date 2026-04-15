#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import asyncio
import json
import time
import argparse
import sys
import os
from datetime import datetime
import struct

# Check availability of websockets library
try:
    import websockets
except ImportError:
    print("Warning: websockets not installed, please use 'pip install websockets' to install")
    sys.exit(1)

# Check availability of opus library
try:
    import opuslib
except ImportError:
    print("Warning: opuslib not installed, please use 'pip install opuslib' to install")
    HAS_OPUS = False
else:
    HAS_OPUS = True

# Helper function for saving logs
def log(message):
    """Log to console and file"""
    print(message)
    with open("websocket_client.log", "a", encoding="utf-8") as f:
        timestamp = time.strftime("%Y-%m-%d %H:%M:%S")
        f.write(f"[{timestamp}] {message}\n")

# Audio parameter constants
AUDIO_RATE = 24000
FRAME_DURATION = 20

# Opus encoding constants
SAMPLE_RATE = 16000
CHANNELS = 1
FRAME_DURATION_MS = 60
PCM_BUFFER_SIZE = SAMPLE_RATE * CHANNELS * FRAME_DURATION_MS // 1000

# Message type constants
MESSAGE_TYPE_HELLO = "hello"
MESSAGE_TYPE_LISTEN = "listen"
MESSAGE_TYPE_ABORT = "abort"
MESSAGE_TYPE_IOT = "iot"

# Message state constants
MESSAGE_STATE_START = "start"
MESSAGE_STATE_STOP = "stop"
MESSAGE_STATE_DETECT = "detect"
MESSAGE_STATE_SUCCESS = "success"
MESSAGE_STATE_ERROR = "error"
MESSAGE_STATE_ABORT = "abort"

# Global variables
opus_data = []
detect_start_ts = 0

async def send_json_message(websocket, msg):
    """Send JSON message"""
    data = json.dumps(msg)
    log(f"Sending message: {data}")
    await websocket.send(data)

async def send_listen_start(websocket, device_id):
    """Send listen start message"""
    listen_start_msg = {
        "type": MESSAGE_TYPE_LISTEN,
        "device_id": device_id,
        "state": MESSAGE_STATE_START,
        "mode": "manual"
    }
    await send_json_message(websocket, listen_start_msg)

async def send_listen_stop(websocket, device_id):
    """Send listen stop message"""
    listen_stop_msg = {
        "type": MESSAGE_TYPE_LISTEN,
        "device_id": device_id,
        "state": MESSAGE_STATE_STOP,
        "mode": "manual"
    }
    await send_json_message(websocket, listen_stop_msg)

async def send_listen_detect(websocket, device_id, text):
    """Send listen detect message"""
    listen_detect_msg = {
        "type": MESSAGE_TYPE_LISTEN,
        "device_id": device_id,
        "state": MESSAGE_STATE_DETECT,
        "text": text
    }
    await send_json_message(websocket, listen_detect_msg)

def save_opus_data():
    """Save Opus data to file"""
    with open("opus_ws.data", "wb") as f:
        for data in opus_data:
            f.write(data)
    log(f"Saved {len(opus_data)} frames of Opus data to opus_ws.data")

def opus_to_wav(opus_data, sample_rate, channels, output_file):
    """Convert Opus data to WAV file
    
    Steps:
    1. Use opuslib to decode opus data to PCM format
    2. Use wave library to write PCM data to WAV file
    """
    log(f"Converting {len(opus_data)} frames of Opus data to WAV file: {output_file}")
    
    if not HAS_OPUS:
        log("opus decoder not installed, please install opuslib: pip install opuslib")
        
        # If opus library is not available, at least save the raw data
        raw_output = output_file + ".opus.raw"
        with open(raw_output, 'wb') as f:
            for data in opus_data:
                f.write(data)
        log(f"Raw opus data saved to: {raw_output}")
        log("To play this audio, please install opuslib and run the program again")
        
        # Create an empty WAV file with only header information
        try:
            import wave
            # Generate 1 second of silence WAV file
            silence = b'\x00' * (sample_rate * channels * 2)
            with wave.open(output_file, 'wb') as wav_file:
                wav_file.setnchannels(channels)
                wav_file.setsampwidth(2)  # 16-bit PCM
                wav_file.setframerate(sample_rate)
                wav_file.writeframes(silence)
            log(f"Created WAV file with silence: {output_file}")
        except Exception as e:
            log(f"Failed to create WAV file: {e}")
        return

    try:
        import wave
        # 1. Create opus decoder
        decoder = opuslib.Decoder(sample_rate, channels)
        
        # Decode all frames to PCM (16-bit little-endian PCM)
        pcm_data = bytearray()
        
        # Decode each frame
        total_input_bytes = 0
        log("\n====== Per-frame Decoding Details ======")
        log("Frame#\tInput Bytes\tOutput Bytes\tSamples\tDuration(ms)")
        log("-----------------------------------------")
        
        for i, frame in enumerate(opus_data):
            try:
                # Calculate max PCM length = sample_rate * channels * max_frame_duration(120ms) / 1000 * 2(bytes/sample)
                max_pcm_size = int(sample_rate * channels * 120 / 1000 * 2)
                
                # Decode opus frame to PCM
                decoded_pcm = decoder.decode(frame, max_pcm_size)
                
                # Calculate output statistics
                input_bytes = len(frame)
                total_input_bytes += input_bytes
                output_bytes = len(decoded_pcm)
                output_samples = output_bytes // (2 * channels)  # 16位PCM
                output_duration_ms = output_samples * 1000 / sample_rate
                
                pcm_data.extend(decoded_pcm)
                
                # Print per-frame information
                log(f"{i}\t{input_bytes}\t\t{output_bytes}\t\t{output_samples}\t\t{output_duration_ms:.2f}")
                
            except Exception as e:
                log(f"Failed to decode frame {i}: {e}")
        
        # Calculate overall statistics
        total_samples = len(pcm_data) // (2 * channels)
        total_duration_ms = total_samples * 1000 / sample_rate
        
        # Print overall statistics
        log("\n====== Overall Statistics ======")
        log(f"Total frames: {len(opus_data)}")
        log(f"Total input bytes: {total_input_bytes}")
        log(f"Total output bytes: {len(pcm_data)}")
        log(f"Total samples: {total_samples}")
        log(f"Total duration: {total_duration_ms:.2f}ms ({total_duration_ms/1000:.2f}s)")
        log("========================\n")
        
        # 2. Write PCM data to WAV file
        # WAV file structure: RIFF header + format chunk + data chunk
        with wave.open(output_file, 'wb') as wav_file:
            wav_file.setnchannels(channels)         # Set number of channels
            wav_file.setsampwidth(2)                # Set sample width to 2 bytes (16-bit)
            wav_file.setframerate(sample_rate)      # Set sample rate
            wav_file.writeframes(bytes(pcm_data))   # Write PCM data
        
        log(f"Audio decoding and conversion completed, total {len(pcm_data)} bytes")
        log(f"Audio duration: {total_duration_ms/1000:.2f} seconds")
        log(f"Audio file saved to: {output_file}")
        
    except Exception as e:
        log(f"Error during conversion: {e}")
        import traceback
        traceback.print_exc()

async def send_text_to_speech(websocket, device_id, text):
    """Call TTS service to generate speech and send"""
    log(f"Sending text to speech: {text}")
    
    # Send listen_detect message
    try:
        await send_listen_detect(websocket, device_id, text)
        log("listen_detect message sent")
    except Exception as e:
        log(f"Failed to send listen_detect message: {e}")
    
    return

async def receive_messages(websocket):
    """Handle messages received from server"""
    global opus_data
    first_recv_frame = False
    recv_interval = 0
    
    try:
        while True:
            try:
                message = await websocket.recv()
                
                if isinstance(message, str):
                    log(f"Received server message: {message}")
                    try:
                        server_msg = json.loads(message)
                        
                        if server_msg.get("type") == "tts" and server_msg.get("state") == "stop":
                            opus_to_wav(opus_data, 24000, 1, "ws_output_24000.wav")
                    except json.JSONDecodeError:
                        log(f"Cannot parse JSON message: {message}")
                
                elif isinstance(message, bytes):
                    now = int(time.time() * 1000)
                    if not first_recv_frame:
                        first_recv_frame = True
                        log(f"First frame arrival time: {now - detect_start_ts} ms")
                    
                    opus_data.append(message)
                    log(f"Received audio data: {len(message)} bytes, interval: {now - recv_interval} ms")
                    recv_interval = now
            except Exception as e:
                log(f"Error processing message: {e}")
                import traceback
                traceback.print_exc()
                break
    
    except websockets.exceptions.ConnectionClosed:
        log("Connection closed")
    except Exception as e:
        log(f"Error receiving message: {e}")
        import traceback
        traceback.print_exc()

async def client(server_addr, device_id, audio_file, text):
    """Run WebSocket client"""
    global opus_data, detect_start_ts
    opus_data = []
    
    log(f"Connecting to server: {server_addr}")
    
    try:
        # Set HTTP headers
        headers = {
            "Device-Id": device_id,
            "Content-Type": "application/json"
        }
        
        # Python 3.6 requires different WebSocket connection method
        async with websockets.connect(server_addr, extra_headers=headers) as websocket:
            log("Connected to server")
            
            # Send hello message
            hello_msg = {
                "type": MESSAGE_TYPE_HELLO,
                "device_id": device_id,
                "transport": "websocket",
                "version": 1,
                "audio_params": {
                    "sample_rate": SAMPLE_RATE,
                    "channels": CHANNELS,
                    "frame_duration": FRAME_DURATION_MS,
                    "format": "opus"
                }
            }
            
            await send_json_message(websocket, hello_msg)
            
            # Start message receiving task - Python 3.6 compatible
            receive_task = asyncio.ensure_future(receive_messages(websocket))
            
            # Wait for server response
            await asyncio.sleep(1)
            
            log("Starting to send audio data...")
            detect_start_ts = int(time.time() * 1000)
            
            # Send text to speech
            try:
                await send_text_to_speech(websocket, device_id, text)
            except Exception as e:
                log(f"Failed to send text to speech: {e}")
                import traceback
                traceback.print_exc()
            
            # Wait for message receiving task to complete
            try:
                await asyncio.sleep(30)  # Wait 30 seconds for server response
                receive_task.cancel()
            except asyncio.CancelledError:
                pass
            except Exception as e:
                log(f"Error waiting for message: {e}")
                import traceback
                traceback.print_exc()
    
    except Exception as e:
        log(f"Client connection error: {e}")
        import traceback
        traceback.print_exc()

def main():
    """Main function"""
    # Check Python version
    py_version = sys.version_info
    log(f"Python version: {py_version.major}.{py_version.minor}.{py_version.micro}")
    
    # Create log file
    with open("websocket_client.log", "w", encoding="utf-8") as f:
        f.write(f"=== Xiaozhi WebSocket Client Log - {time.strftime('%Y-%m-%d %H:%M:%S')} ===\n")
    
    parser = argparse.ArgumentParser(description="Xiaozhi WebSocket Client")
    parser.add_argument("--server", "-s", default="ws://localhost:8989/xiaozhi/v1/", help="Server address")
    parser.add_argument("--device", "-d", default="test-device-001", help="Device ID")
    parser.add_argument("--audio", "-a", default="../test.wav", help="Audio file path")
    parser.add_argument("--text", "-t", default="Hello test", help="Text")
    
    args = parser.parse_args()
    
    log(f"Running Xiaozhi client\nServer: {args.server}\nDevice ID: {args.device}\nAudio file: {args.audio}\nText: {args.text}")
    
    try:
        # Compatible with different Python versions
        loop = asyncio.get_event_loop()
        loop.run_until_complete(client(args.server, args.device, args.audio, args.text))
        loop.close()
    except Exception as e:
        log(f"Client run failed: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    main()
