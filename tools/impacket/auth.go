// tools/impacket/auth.go
package impacket

// authArgs builds the positional credential argument used by most impacket tools:
//
//	domain/username:password@target
//	domain/username@target -hashes LMHASH:NTHASH
//
// If domain is empty, the domain/ prefix is omitted.
// If hash is non-empty, password is ignored and -hashes is appended.
func authArgs(domain, username, password, hash, target string) []string {
	cred := username
	if domain != "" {
		cred = domain + "/" + username
	}
	if hash == "" && password != "" {
		cred += ":" + password
	}
	cred += "@" + target

	args := []string{cred}
	if hash != "" {
		args = append(args, "-hashes", hash)
	}
	return args
}
