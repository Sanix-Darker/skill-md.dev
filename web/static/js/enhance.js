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
  let currentTarget = 'output'; // 'output' or 'input'

  // DOM elements
  let enhanceBtn = null;
  let statusEl = null;
  let modeSelect = null;
  let inputAssistBtn = null;
  let inputAssistMode = null;
  let inputAssistStatus = null;

  /**
   * Initialize the enhancement system
   */
  function init() {
    // Find UI elements for output enhancement
    enhanceBtn = document.getElementById('enhance-btn');
    statusEl = document.getElementById('enhance-status');
    modeSelect = document.getElementById('enhance-mode');

    // Find UI elements for input assist
    inputAssistBtn = document.getElementById('input-assist-btn');
    inputAssistMode = document.getElementById('input-assist-mode');
    inputAssistStatus = document.getElementById('input-assist-status');

    // Set up event listeners for output enhancement
    if (enhanceBtn) {
      enhanceBtn.addEventListener('click', handleEnhanceClick);
    }

    // Set up event listeners for input assist
    if (inputAssistBtn) {
      inputAssistBtn.addEventListener('click', handleInputAssistClick);
    }

    // Check for WebWorker and module support
    if (!window.Worker) {
      if (enhanceBtn) {
        showStatus('Browser does not support WebWorkers', 'error');
        enhanceBtn.disabled = true;
      }
      if (inputAssistBtn) {
        inputAssistBtn.disabled = true;
        inputAssistBtn.classList.add('opacity-50');
      }
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
        worker = new Worker('/static/js/llm-worker.js', { type: 'module' });

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
    const isInputMode = currentTarget === 'input';

    switch (data.type) {
      case 'loading':
        isLoading = true;
        if (isInputMode) {
          showInputAssistStatus(data.message, 'loading');
        } else {
          showStatus(data.message, 'loading', data.progress);
        }
        updateButtonState();
        break;

      case 'ready':
        isReady = true;
        isLoading = false;
        if (isInputMode) {
          showInputAssistStatus('AI ready', 'success');
        } else {
          showStatus('AI ready', 'success');
        }
        updateButtonState();
        break;

      case 'processing':
        isProcessing = true;
        if (isInputMode) {
          showInputAssistStatus(data.message, 'loading');
        } else {
          showStatus(data.message, 'loading');
        }
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
        if (isInputMode) {
          showInputAssistStatus(data.message, 'error');
        } else {
          showStatus(data.message, 'error');
        }
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

    currentTarget = 'output';

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
   * Handle input assist button click
   */
  async function handleInputAssistClick() {
    if (isProcessing || isLoading) {
      return;
    }

    // Get input content
    const textarea = document.querySelector('textarea[name="content"]');
    if (!textarea || !textarea.value.trim()) {
      showInputAssistStatus('No input content to assist with', 'error');
      return;
    }

    const content = textarea.value;
    const mode = inputAssistMode?.value || 'format';
    currentTarget = 'input';

    try {
      // Initialize worker if needed
      if (!worker) {
        showInputAssistStatus('Loading AI model...', 'loading');
        await initWorker();
      }

      // Wait for model to be ready
      if (!isReady) {
        showInputAssistStatus('Waiting for model to load...', 'loading');
        return;
      }

      // Build input-specific prompt
      const inputPrompt = buildInputAssistPrompt(content, mode);

      // Send enhancement request
      worker.postMessage({
        type: 'enhance',
        payload: {
          content: inputPrompt,
          mode: 'input_' + mode,
          maxTokens: 768
        }
      });

    } catch (error) {
      showInputAssistStatus('Assist failed: ' + error.message, 'error');
    }
  }

  /**
   * Build prompt for input assistance
   */
  function buildInputAssistPrompt(content, mode) {
    const prompts = {
      format: `Clean up and format this API specification. Fix any syntax errors, correct indentation, and ensure proper structure. Return only the corrected specification:

${content}

Corrected specification:`,

      expand: `Expand this abbreviated or incomplete API specification with more detail. Add missing fields, descriptions, and examples where appropriate. Return the expanded specification:

${content}

Expanded specification:`,

      validate: `Review this API specification for common issues. List any problems found and suggest fixes. Be concise:

${content}

Issues and suggestions:`
    };

    return prompts[mode] || prompts.format;
  }

  /**
   * Show input assist status
   */
  function showInputAssistStatus(message, type) {
    if (!inputAssistStatus) return;

    inputAssistStatus.classList.remove('hidden');
    inputAssistStatus.textContent = message;
    inputAssistStatus.classList.remove('text-terminal-accent', 'text-red-400', 'pulse');

    switch (type) {
      case 'success':
        inputAssistStatus.classList.add('text-terminal-accent');
        break;
      case 'error':
        inputAssistStatus.classList.add('text-red-400');
        break;
      case 'loading':
        inputAssistStatus.classList.add('text-terminal-muted', 'pulse');
        break;
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
    // Check if this is an input assist result
    if (mode && mode.startsWith('input_')) {
      handleInputAssistResult(content, mode.replace('input_', ''));
      return;
    }

    // Find where to put the result (output enhancement)
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

    currentTarget = 'output';
    showStatus('Enhancement complete!', 'success');

    // Hide status after a moment
    setTimeout(() => {
      if (statusEl) {
        statusEl.classList.add('hidden');
      }
    }, 2000);
  }

  /**
   * Handle input assist result
   */
  function handleInputAssistResult(content, mode) {
    const textarea = document.querySelector('textarea[name="content"]');
    if (!textarea) return;

    if (mode === 'validate') {
      // For validation mode, show suggestions without replacing content
      showInputAssistStatus('See suggestions below', 'success');

      // Create or update suggestions container
      let suggestionsEl = document.getElementById('input-suggestions');
      if (!suggestionsEl) {
        suggestionsEl = document.createElement('div');
        suggestionsEl.id = 'input-suggestions';
        suggestionsEl.className = 'mt-2 p-3 bg-terminal-bg border border-terminal-accent text-xs text-terminal-text fade-in';
        textarea.parentNode.insertBefore(suggestionsEl, textarea.nextSibling);
      }
      suggestionsEl.innerHTML = '<strong class="text-terminal-accent">AI Suggestions:</strong><pre class="mt-1 whitespace-pre-wrap">' + escapeHtml(content) + '</pre>';

      // Auto-hide after 10 seconds
      setTimeout(() => {
        if (suggestionsEl) {
          suggestionsEl.remove();
        }
      }, 10000);
    } else {
      // For format/expand modes, replace the textarea content
      textarea.value = content;

      // Trigger flash animation on textarea
      textarea.classList.add('flash-success');
      setTimeout(() => textarea.classList.remove('flash-success'), 500);

      showInputAssistStatus('Input updated!', 'success');
    }

    currentTarget = 'output';

    // Hide status after a moment
    setTimeout(() => {
      if (inputAssistStatus) {
        inputAssistStatus.classList.add('hidden');
      }
    }, 3000);
  }

  /**
   * Escape HTML for safe display
   */
  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
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
    // Update enhance button
    if (enhanceBtn) {
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

    // Update input assist button
    if (inputAssistBtn) {
      if (isLoading || isProcessing) {
        inputAssistBtn.disabled = true;
        inputAssistBtn.classList.add('opacity-50');
      } else {
        inputAssistBtn.disabled = false;
        inputAssistBtn.classList.remove('opacity-50');
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
