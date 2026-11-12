package internal

// Independent native crash qualification only. Nil in production; no request,
// environment variable or public adapter can configure this callback.
var toolStorageAuthoringTestPhase func(string)
