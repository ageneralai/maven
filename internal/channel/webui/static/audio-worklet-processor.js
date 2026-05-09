// AudioWorklet capture: downsample to 16 kHz mono int16 (Deepgram live linear16).
// Loaded via audioContext.audioWorklet.addModule('/audio-worklet-processor.js').
class MavenPcmCaptureProcessor extends AudioWorkletProcessor {
  process(inputs) {
    const input = inputs[0];
    if (!input || input.length === 0) {
      return true;
    }
    const channel = input[0];
    if (!channel || channel.length === 0) {
      return true;
    }
    const inRate = globalThis.sampleRate;
    const outRate = 16000;
    const ratio = inRate / outRate;
    const outLen = Math.floor(channel.length / ratio);
    if (outLen <= 0) {
      return true;
    }
    const out = new Int16Array(outLen);
    for (let i = 0; i < outLen; i++) {
      const x = channel[Math.floor(i * ratio)] || 0;
      const clamped = Math.max(-1, Math.min(1, x));
      out[i] = clamped < 0 ? clamped * 0x8000 : clamped * 0x7fff;
    }
    this.port.postMessage(out.buffer, [out.buffer]);
    return true;
  }
}
registerProcessor("maven-pcm-capture", MavenPcmCaptureProcessor);
