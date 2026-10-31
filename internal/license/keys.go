package license

import "embed"

// embedded holds the public keys this build trusts, one keys/<id>.pub file per signing key. Keeping retired keys
// keeps the licenses they signed working; deleting one revokes every license it signed.
//
//go:embed keys
var embedded embed.FS

// Trusted returns the public keys built into ATO.
func Trusted() (Keys, error) { return LoadKeys(embedded, "keys") }
