// Optional pure caller transport; emit(string) creates one separate text block.
function forwardNativeOutput(result, emit, options) {
  const object = x => x !== null && typeof x === "object" && !Array.isArray(x);
  const state = { status: "unverified", forwarded: false, exitCode: null };
  const finish = (reason, metadata) => {
    state.reason = reason;
    if (typeof emit === "function") emit(JSON.stringify({
      nativeOutputStatus: state.status, reason, exitCode: state.exitCode,
      forwarded: state.forwarded, ...(metadata ? { nativeMetadata: metadata } : {})
    }));
    return state;
  };
  if (typeof emit !== "function" || !object(options) ||
      !["context", "native-json"].includes(options.mode) ||
      !Number.isSafeInteger(options.maxOutputTokens) ||
      options.maxOutputTokens < 1 || options.maxOutputTokens > 1000000)
    return finish("invalid-options");
  if (!object(result)) return finish("unknown-envelope");
  if (result.session_id != null || result.sessionId != null) {
    state.status = "pending";
    state.pendingResult = result;
    return finish("session-pending");
  }
  if (!Number.isSafeInteger(result.exit_code) || typeof result.output !== "string")
    return finish("missing-finished-output");
  state.exitCode = result.exit_code;
  const body = result.output;
  if (body.length > 4000000 || result.truncated === true ||
      result.output_truncated === true || result.isTruncated === true)
    return finish("truncated");
  if (!Number.isSafeInteger(result.original_token_count) || result.original_token_count < 0)
    return finish("truncation-unverified");
  if (result.original_token_count > options.maxOutputTokens) return finish("truncated");
  let response;
  try { response = JSON.parse(body.replace(/^\uFEFF/, "")); }
  catch (_) { return finish(/Warning: truncated output|tokens truncated|characters truncated/i.test(body) ?
    "truncated" : "invalid-native-json"); }
  if (!object(response) || response.schemaVersion !== 1 ||
      typeof response.ok !== "boolean" || typeof response.code !== "string" ||
      !/^[A-Z][A-Z0-9_]{0,127}$/.test(response.code) ||
      (result.exit_code !== 0 && response.ok))
    return finish("invalid-native-response");
  if (options.mode === "context" && response.ok) {
    if (response.code !== "CONTEXT_COMPACT" || !object(response.data) ||
        response.data.complete !== true || response.data.inputFreshness !== "verified" ||
        typeof response.data.context !== "string")
      return finish("invalid-context-response");
    const { context, ...data } = response.data;
    const metadata = { ...response, data };
    state.native = response;
    state.status = "finished";
    state.forwarded = true;
    emit(context);
    return finish("observed-finished-response", metadata);
  }
  state.native = response;
  state.status = response.ok && result.exit_code === 0 ? "finished" : "refused";
  state.forwarded = true;
  emit(body);
  return finish("observed-finished-response");
}

if (typeof module !== "undefined" && module.exports) module.exports = { forwardNativeOutput };
