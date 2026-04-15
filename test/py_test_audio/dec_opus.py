#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import sys
import os
import wave
import struct
import opuslib

def decode_raw_opus(opus_data, sample_rate=24000, channels=1, frame_size_ms=60):
    """Decode raw Opus data, return PCM data"""
    # Calculate number of samples per frame
    frame_size = int(sample_rate * frame_size_ms / 1000)
    
    # Create decoder
    decoder = opuslib.Decoder(sample_rate, channels)
    
    # Try to decode entire file directly
    try:
        pcm_data = bytearray()
        decoded = decoder.decode(opus_data, frame_size, False)
        for sample in decoded:
            pcm_data.extend(struct.pack('<h', sample))
        return pcm_data
    except Exception as e:
        print(f"Direct decoding failed: {e}")
        return None

def main():
    # Check command line arguments
    if len(sys.argv) < 2:
        print("Usage: python dec_opus.py <opus_file>")
        return

    opus_file = sys.argv[1]
    
    # Check if file exists
    if not os.path.exists(opus_file):
        print(f"Error: file '{opus_file}' does not exist")
        return
    
    # Initialize parameters
    sample_rate = 24000  # Sample rate 24000Hz
    channels = 1         # Mono
    frame_size_ms = 60   # Frame size 60ms
    
    # Read entire opus file content
    with open(opus_file, 'rb') as f:
        opus_data = f.read()
    
    print(f"Read raw Opus data: {len(opus_data)} bytes")
    
    # Decode data
    pcm_data = decode_raw_opus(opus_data, sample_rate, channels, frame_size_ms)
    
    if pcm_data is None or len(pcm_data) == 0:
        print("Decoding failed, failed to generate PCM data")
        return
    
    # Calculate PCM data length (number of samples)
    pcm_samples_count = len(pcm_data) // 2  # 2 bytes per sample
    pcm_duration_ms = pcm_samples_count * 1000 / sample_rate
    
    print(f"Decoded PCM data size: {len(pcm_data)} bytes")
    print(f"PCM samples: {pcm_samples_count}")
    print(f"PCM duration: {pcm_duration_ms:.2f} milliseconds")

if __name__ == "__main__":
    main()
