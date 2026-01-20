/**
 * LLM Enhancement Coordinator
 * Manages browser-based LLM for content enhancement
 */

(function() {
  'use strict';

  // State
  let worker = null;
  let isReady = false;
  let isLoading = false;
  let isProcessing = false;

  // DOM elements
  let enhanceBtn = null;
  let statusEl = null;
  let modeSelect = null;

  /**
   * Initialize the enhancement system
   */
  function init() {
    // Find UI elements
    enhanceBtn = document.getElementById('enhance-btn');
    statusEl = document.getElementById('enhance-status');
    modeSelect = document.getElementById('enhance-mode');

    if (!enhanceBtn) {
      return; // Enhancement not available on this page
    }

    // Set up event listeners
    enhanceBtn.addEventListener('click', handleEnhanceClick);

    // Check for WebWorker and module support
    if (!window.Worker) {
      showStatus('Browser does not support WebWorkers', 'error');
      enhanceBtn.disabled = true;
      return;
    }
  }

  /**
   * Initialize the worker lazily
   */
  function initWorker() {
    if (worker) return Promise.resolve();

    return new Promise((resolve, reject) => {
      try {
        worker = new Worker('/static/js/llm-worker.js');

        worker.onmessage = function(e) {
          handleWorkerMessage(e.data);
          if (e.data.type === 'ready') {
            resolve();
          }
        };

        worker.onerror = function(error) {
          showStatus('Worker error: ' + error.message, 'error');
          reject(error);
        };

        // Wait for worker-ready, then init model
        const readyHandler = function(e) {
          if (e.data.type === 'worker-ready') {
            worker.removeEventListener('message', readyHandler);
            worker.postMessage({ type: 'init' });
          }
        };
        worker.addEventListener('message', readyHandler);

      } catch (error) {
        showStatus('Failed to create worker: ' + error.message, 'error');
        reject(error);
      }
    });
  }

  /**
   * Handle messages from the worker
   */
  function handleWorkerMessage(data) {
    switch (data.type) {
      case 'loading':
        isLoading = true;
        showStatus(data.message, 'loading', data.progress);
        updateButtonState();
        break;

      case 'ready':
        isReady = true;
        isLoading = false;
        showStatus('AI ready', 'success');
        updateButtonState();
        break;

      case 'processing':
        isProcessing = true;
        showStatus(data.message, 'loading');
        updateButtonState();
        break;

      case 'result':
        isProcessing = false;
        handleEnhancementResult(data.content, data.mode);
        updateButtonState();
        break;

      case 'error':
        isProcessing = false;
        isLoading = false;
        showStatus(data.message, 'error');
        updateButtonState();
        break;
    }
  }

  /**
   * Handle enhance button click
   */
  async function handleEnhanceClick() {
    if (isProcessing || isLoading) {
      return;
    }

    // Get content to enhance
    const content = getContentToEnhance();
    if (!content) {
      showStatus('No content to enhance', 'error');
      return;
    }

    // Get enhancement mode
    const mode = modeSelect?.value || 'elaborate';

    try {
      // Initialize worker if needed
      if (!worker) {
        showStatus('Loading AI model...', 'loading');
        await initWorker();
      }

      // Wait for model to be ready
      if (!isReady) {
        showStatus('Waiting for model to load...', 'loading');
        return;
      }

      // Send enhancement request
      worker.postMessage({
        type: 'enhance',
        payload: {
          content: content,
          mode: mode,
          maxTokens: 512
        }
      });

    } catch (error) {
      showStatus('Enhancement failed: ' + error.message, 'error');
    }
  }

  /**
   * Get content to enhance from the page
   */
  function getContentToEnhance() {
    // Try to get from preview area
    const previewContent = document.querySelector('.code-preview pre');
    if (previewContent) {
      return previewContent.textContent;
    }

    // Try to get from textarea
    const textarea = document.querySelector('textarea[name="content"]');
    if (textarea && textarea.value) {
      return textarea.value;
    }

    // Try to get from result area
    const resultArea = document.getElementById('result');
    if (resultArea) {
      return resultArea.textContent;
    }

    return null;
  }

  /**
   * Handle enhancement result
   */
  function handleEnhancementResult(content, mode) {
    // Find where to put the result
    const previewContent = document.querySelector('.code-preview pre');
    if (previewContent) {
      // Append or replace content
      const modeLabels = {
        elaborate: 'Enhanced Content',
        examples: 'Added Examples',
        bestPractices: 'Best Practices',
        simplify: 'Simplified Content'
      };

      const divider = '\n\n---\n\n## ' + (modeLabels[mode] || 'AI Enhancement') + '\n\n';
      previewContent.textContent += divider + content;

      // Trigger flash animation
      const container = previewContent.closest('.code-preview');
      if (container) {
        container.classList.add('flash-success');
        setTimeout(() => container.classList.remove('flash-success'), 500);
      }
    }

    showStatus('Enhancement complete!', 'success');

    // Hide status after a moment
    setTimeout(() => {
      if (statusEl) {
        statusEl.classList.add('hidden');
      }
    }, 2000);
  }

  /**
   * Show status message
   */
  function showStatus(message, type, progress) {
    if (!statusEl) return;

    statusEl.classList.remove('hidden');
    statusEl.textContent = message;

    // Update styling based on type
    statusEl.classList.remove('text-terminal-accent', 'text-red-400', 'pulse');

    switch (type) {
      case 'success':
        statusEl.classList.add('text-terminal-accent');
        break;
      case 'error':
        statusEl.classList.add('text-red-400');
        break;
      case 'loading':
        statusEl.classList.add('text-terminal-muted', 'pulse');
        break;
    }

    // Show progress if available
    if (progress !== undefined) {
      statusEl.textContent = message;
    }
  }

  /**
   * Update button state
   */
  function updateButtonState() {
    if (!enhanceBtn) return;

    if (isLoading || isProcessing) {
      enhanceBtn.disabled = true;
      enhanceBtn.classList.add('opacity-50');

      // Update button text
      const btnText = enhanceBtn.querySelector('.btn-text');
      if (btnText) {
        btnText.textContent = isLoading ? 'Loading...' : 'Enhancing...';
      }
    } else {
      enhanceBtn.disabled = false;
      enhanceBtn.classList.remove('opacity-50');

      const btnText = enhanceBtn.querySelector('.btn-text');
      if (btnText) {
        btnText.textContent = 'Enhance with AI';
      }
    }
  }

  // Initialize when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  // Expose for debugging
  window.SkillForgeEnhance = {
    init: init,
    getStatus: () => ({ isReady, isLoading, isProcessing })
  };

  // Toggle enhance info panel
  window.toggleEnhanceInfo = function() {
    const infoEl = document.getElementById('enhance-info');
    if (infoEl) {
      infoEl.classList.toggle('hidden');
    }
  };

})();
