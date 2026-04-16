#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <opus/opus.h>

// PCM sample count corresponding to 24000Hz sample rate, mono channel, 60ms frame length
#define SAMPLE_RATE 24000
#define CHANNELS 1
#define FRAME_SIZE_MS 60
#define FRAME_SIZE (SAMPLE_RATE * FRAME_SIZE_MS / 1000)

// Maximum bytes per frame (safe value)
#define MAX_PACKET_SIZE 1500

int main(int argc, char *argv[]) {
    if (argc < 2) {
        printf("Usage: %s <opus_file> [raw]\n", argv[0]);
        printf("Parameters:\n");
        printf("  <opus_file>: Path to the opus file to decode\n");
        printf("  [raw]: Optional parameter, specify as raw to process raw opus data without length prefix\n");
        return 1;
    }

    // Check if in raw mode
    int raw_mode = 1;

    // Open opus file
    FILE *fp = fopen(argv[1], "rb");
    if (!fp) {
        printf("Cannot open file: %s\n", argv[1]);
        return 1;
    }

    // Get file size
    fseek(fp, 0, SEEK_END);
    long file_size = ftell(fp);
    fseek(fp, 0, SEEK_SET);

    // Read entire file content
    unsigned char *opus_data = (unsigned char *)malloc(file_size);
    if (!opus_data) {
        printf("Memory allocation failed\n");
        fclose(fp);
        return 1;
    }
    
    size_t bytes_read = fread(opus_data, 1, file_size, fp);
    fclose(fp);
    
    printf("File read successfully, size: %ld bytes\n", bytes_read);

    // Create opus decoder
    int error;
    OpusDecoder *decoder = opus_decoder_create(SAMPLE_RATE, CHANNELS, &error);
    if (error != OPUS_OK) {
        printf("Failed to create opus decoder: %s\n", opus_strerror(error));
        free(opus_data);
        return 1;
    }
    
    printf("Decoder created successfully, sample rate: %d Hz, channels: %d\n", SAMPLE_RATE, CHANNELS);
    printf("Theoretical PCM samples per frame (60ms): %d\n", FRAME_SIZE);

    // Prepare PCM output buffer - theoretically 60ms@24000Hz should have 1440 sample points
    opus_int16 pcm[FRAME_SIZE * CHANNELS];

    int frame_count = 0;
    
    // Try to decode the entire file as a single opus frame
    int samples = opus_decode(decoder, opus_data, bytes_read, pcm, FRAME_SIZE, 0);
    
    if (samples < 0) {
        printf("Decoding failed: %s\n", opus_strerror(samples));
    } else {
        frame_count++;
        printf("Decoding completed: opus length %ld bytes, decoded PCM samples %d\n", bytes_read, samples);
        
        // Can save PCM to file
        char output_file[256];
        sprintf(output_file, "%s.pcm", argv[1]);
        FILE *out_fp = fopen(output_file, "wb");
        if (out_fp) {
            fwrite(pcm, sizeof(opus_int16), samples, out_fp);
            fclose(out_fp);
            printf("PCM data saved to %s\n", output_file);
        }
    }
    
    printf("Total decoded %d frames\n", frame_count);
    
    // Cleanup resources
    opus_decoder_destroy(decoder);
    free(opus_data);
    
    return 0;
}
