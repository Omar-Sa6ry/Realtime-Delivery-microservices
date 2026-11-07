package grpc

// TranslateError exposes the package-private translateError helper to the
// external test package (grpc_test). This file only exists in test builds.
//
// Every translateError branch except the nil-error guard is reachable through
// the RPC handlers; no handler ever calls it with a nil error, so the external
// tests invoke it directly to cover that guard (and to double-check the
// code/message mapping table).
var TranslateError = translateError
