Alpine.data("chatInput", function () {
  const SpeechRecognition =
    window.SpeechRecognition || window.webkitSpeechRecognition;

  return {
    hasText: false,
    isListening: false,
    hasSpeechRecognition: !!SpeechRecognition,
    _recognition: null,
    _audioCtx: null,
    _analyser: null,
    _rafId: null,

    get isStreaming() {
      return Alpine.store("chat").isStreaming;
    },

    _stopListening() {
      if (this._recognition) this._recognition.stop();
    },

    _clearListening() {
      this._stopListening();
      this.$refs.textarea.value = "";
      this.hasText = false;
    },

    _startVisualiser(stream) {
      this._audioCtx = new AudioContext();
      this._analyser = this._audioCtx.createAnalyser();
      this._analyser.fftSize = 256;
      this._audioCtx.createMediaStreamSource(stream).connect(this._analyser);
      const data = new Uint8Array(this._analyser.frequencyBinCount);
      const bars = this.$refs.voiceBars.querySelectorAll("span");
      // voice fundamentals sit in roughly the first quarter of bins; spread bars across that
      const activeRange = Math.floor(data.length / 4);
      const step = Math.floor(activeRange / bars.length);
      const tick = () => {
        this._analyser.getByteFrequencyData(data);
        bars.forEach((bar, i) => {
          const v = Math.max(data[i * step] / 255, 0.08);
          bar.style.setProperty("--bar-h", v);
        });
        this._rafId = requestAnimationFrame(tick);
      };
      this._rafId = requestAnimationFrame(tick);
    },

    _stopVisualiser() {
      if (this._rafId) {
        cancelAnimationFrame(this._rafId);
        this._rafId = null;
      }
      if (this._audioCtx) {
        this._audioCtx.close();
        this._audioCtx = null;
        this._analyser = null;
      }
    },

    _startListening() {
      const rec = new SpeechRecognition();
      rec.continuous = true;
      rec.interimResults = true;
      rec.lang = navigator.language || "en-US";

      let speechBase = "";

      rec.onstart = () => {
        this.isListening = true;
        speechBase = this.$refs.textarea.value;
      };

      rec.onresult = (e) => {
        let newFinal = "";
        let interim = "";
        for (let i = e.resultIndex; i < e.results.length; i++) {
          if (e.results[i].isFinal) {
            newFinal += e.results[i][0].transcript;
          } else {
            interim += e.results[i][0].transcript;
          }
        }
        if (newFinal) speechBase += newFinal;
        const value = speechBase + interim;
        this.$refs.textarea.value = value;
        this.hasText = value.length > 0;
      };

      rec.onerror = () => {
        this._stopVisualiser();
        this.isListening = false;
        this._recognition = null;
      };

      rec.onend = () => {
        this._stopVisualiser();
        this.isListening = false;
        this._recognition = null;
      };

      this._recognition = rec;
      rec.start();

      navigator.mediaDevices
        .getUserMedia({ audio: true })
        .then((stream) => this._startVisualiser(stream))
        .catch(() => {});
    },

    micClicked() {
      if (this.isListening) {
        this._clearListening();
      } else {
        this._startListening();
      }
    },

    sendClicked() {
      if (this.isStreaming) this.$dispatch("chat-stop");
      else if (this.isListening) this._stopListening();
    },

    textareaEnterClicked(e) {
      if (!e.shiftKey && !this.$refs.textarea.value.includes("\n")) {
        e.preventDefault();
        this.$refs.form.requestSubmit();
      }
    },

    submitForm(e) {
      e.preventDefault();
      this._stopListening();
      const text = this.$refs.textarea.value.trim();
      if (!text) return;
      this.$refs.textarea.value = "";
      this.hasText = false;
      this.$dispatch("chat-submit", { text });
    },

    get micState() {
      if (this.isListening) {
        return {
          ariaLabel: "Cancel dictation",
          class: "btn btn-icon",
          icon: "close",
        };
      }
      if (this.hasText) {
        return {
          ariaLabel: "Dictate message",
          class: "btn btn-icon",
          icon: "mic",
        };
      }
      return {
        ariaLabel: "Dictate message",
        class: "btn-primary",
        icon: "mic",
      };
    },

    get sendState() {
      if (this.isStreaming) {
        return {
          ariaLabel: "Stop response",
          type: "button",
          class: "btn-tertiary icon-fill-1",
          icon: "stop",
        };
      }
      if (this.isListening) {
        return {
          ariaLabel: "Send message",
          type: "button",
          class: "btn-tertiary icon-fill-1",
          icon: "stop",
        };
      }
      if (this.hasText) {
        return {
          ariaLabel: "Send message",
          type: "submit",
          class: "btn-primary",
          icon: "send",
        };
      }
      return {
        ariaLabel: "Send message",
        type: "submit",
        class: "btn btn-icon",
        icon: "send",
      };
    },

    get totalState() {
      return {
        show:
          !!this.$store.chat.totalCost &&
          !this.$store.chat.isStreaming &&
          !this.isListening,
        totalCost: this.$store.chat.totalCost,
      };
    },
  };
});

Alpine.data("chatBottomsheet", function () {
  return {
    get _totals() {
      return Alpine.store("chat").totals;
    },

    get totalCost() {
      const t = this._totals;
      if (!t) return "$0.000000";
      return t.cost_usd < 0.000001 ? "<$0.000001" : "$" + t.cost_usd.toFixed(6);
    },

    get totalMessages() {
      const count = this._totals?.message_count ?? 0;
      return count === 1 ? "1 message" : `${count} messages`;
    },

    get contextWindow() {
      return (this._totals?.context_window ?? 0).toLocaleString();
    },

    get contextWindowUsage() {
      return Alpine.store("chat").contextPercent;
    },

    get percentOfContextWindow() {
      return Alpine.store("chat").contextPercent + "% used";
    },

    get contextUsedTokens() {
      return (this._totals?.context_used_tokens ?? 0).toLocaleString();
    },

    get totalInputTokens() {
      return (this._totals?.input_tokens ?? 0).toLocaleString();
    },

    get totalOutputTokens() {
      return (this._totals?.output_tokens ?? 0).toLocaleString();
    },

    get totalCacheRead() {
      return (this._totals?.cache_read_input_tokens ?? 0).toLocaleString();
    },

    get averageResponseTime() {
      const ms = this._totals?.avg_response_ms;
      if (!ms) return "—";
      return ms >= 1000 ? (ms / 1000).toFixed(1) + " s" : ms + " ms";
    },
  };
});
