/**
 * LLM WebWorker for browser-based content enhancement
 * Uses Transformers.js to run SmolLM2-360M-Instruct locally
 */

// Import Transformers.js from CDN
importScripts('https://cdn.jsdelivr.net/npm/@huggingface/transformers@3.5.1');

let generator = null;
let isLoading = false;

// Model configuration
const MODEL_CONFIG = {
  default: 'Xenova/SmolLM2-360M-Instruct',
  quality: 'Xenova/Phi-3-mini-4k-instruct'
};

// Handle messages from main thread
self.onmessage = async function(e) {
  const { type, payload } = e.data;

  switch (type) {
    case 'init':
      await initModel(payload?.model || 'default');
      break;
    case 'enhance':
      await enhanceContent(payload);
      break;
    case 'abort':
      // Future: implement abort functionality
      break;
    case 'status':
      self.postMessage({
        type: 'status',
        ready: generator !== null,
        loading: isLoading
      });
      break;
  }
};

/**
 * Initialize the LLM model
 */
async function initModel(modelType) {
  if (generator) {
    self.postMessage({ type: 'ready', cached: true });
    return;
  }

  if (isLoading) {
    return;
  }

  isLoading = true;
  const modelId = MODEL_CONFIG[modelType] || MODEL_CONFIG.default;

  try {
    self.postMessage({ type: 'loading', message: 'Initializing model...' });

    // Configure Transformers.js
    const { pipeline, env } = await import('https://cdn.jsdelivr.net/npm/@huggingface/transformers@3.5.1');

    // Allow local models and set cache
    env.allowLocalModels = true;
    env.useBrowserCache = true;

    self.postMessage({ type: 'loading', message: `Loading ${modelId}...` });

    generator = await pipeline('text-generation', modelId, {
      progress_callback: (progress) => {
        if (progress.status === 'progress') {
          const pct = Math.round((progress.loaded / progress.total) * 100);
          self.postMessage({
            type: 'loading',
            message: `Downloading model... ${pct}%`,
            progress: pct
          });
        }
      }
    });

    isLoading = false;
    self.postMessage({ type: 'ready', model: modelId });

  } catch (error) {
    isLoading = false;
    self.postMessage({
      type: 'error',
      message: `Failed to load model: ${error.message}`
    });
  }
}

/**
 * Enhance content using the loaded model
 */
async function enhanceContent(payload) {
  if (!generator) {
    self.postMessage({
      type: 'error',
      message: 'Model not initialized. Please wait for loading to complete.'
    });
    return;
  }

  const { content, mode, maxTokens = 512 } = payload;

  try {
    self.postMessage({ type: 'processing', message: 'Generating enhancement...' });

    // Build prompt based on mode
    const prompt = buildPrompt(content, mode);

    // Generate response
    const result = await generator(prompt, {
      max_new_tokens: maxTokens,
      temperature: 0.7,
      top_p: 0.9,
      do_sample: true,
      return_full_text: false
    });

    const generatedText = result[0].generated_text;

    self.postMessage({
      type: 'result',
      content: cleanupResponse(generatedText, mode),
      mode: mode
    });

  } catch (error) {
    self.postMessage({
      type: 'error',
      message: `Enhancement failed: ${error.message}`
    });
  }
}

/**
 * Build prompt based on enhancement mode
 */
function buildPrompt(content, mode) {
  const prompts = {
    elaborate: `You are a technical documentation expert. Enhance this API documentation with more detail, context, and clarity. Add explanations where helpful.

Documentation to enhance:
${content}

Enhanced documentation:`,

    examples: `You are a developer advocate. Add practical code examples to this API documentation. Include examples in multiple languages (curl, JavaScript, Python).

Documentation:
${content}

Documentation with examples:`,

    bestPractices: `You are a senior API architect. Add best practices, recommendations, and common pitfalls to this API documentation.

Documentation:
${content}

Documentation with best practices:`,

    simplify: `You are a technical writer. Simplify this API documentation to be more accessible to beginners. Use clear, simple language.

Documentation:
${content}

Simplified documentation:`
  };

  return prompts[mode] || prompts.elaborate;
}

/**
 * Clean up the generated response
 */
function cleanupResponse(text, mode) {
  // Remove any repeated prompt content
  let cleaned = text.trim();

  // Remove common artifacts
  cleaned = cleaned.replace(/^(Enhanced documentation:|Documentation with.*?:|Simplified documentation:)\s*/i, '');

  // Ensure proper markdown formatting
  if (!cleaned.startsWith('#') && !cleaned.startsWith('-') && !cleaned.startsWith('*')) {
    // Don't add headers if already well-formatted
  }

  return cleaned;
}

// Notify main thread that worker is ready
self.postMessage({ type: 'worker-ready' });
