package ld

import (
	"net/netip"
	"strings"
	"unicode/utf8"
)

// wellFormedIRI reports whether s matches the IRI rule of the RFC 3987 §2.2
// ABNF: an absolute IRI with a scheme. JSON-LD 1.1 API §8.1 lets toRdf emit
// only well-formed IRIs, which is also what keeps a character such as a space
// or ">" from ending an N-Quads term early.
func wellFormedIRI(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	u := splitURIReference(s)
	if !u.hasScheme || !validScheme(u.scheme) {
		return false
	}
	if u.hasAuthority {
		if !validAuthority(u.authority) {
			return false
		}
	} else if strings.HasPrefix(u.path, "//") {
		return false
	}
	return validIRIChars(u.path, "/:@", false) &&
		(!u.hasQuery || validIRIChars(u.query, "/?:@", true)) &&
		(!u.hasFragment || validIRIChars(u.fragment, "/?:@", false))
}

func validScheme(scheme string) bool {
	for i, c := range scheme {
		isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !isAlpha && (i == 0 || !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.') {
			return false
		}
	}
	return scheme != ""
}

// validAuthority checks iauthority = [ iuserinfo "@" ] ihost [ ":" port ].
func validAuthority(authority string) bool {
	hostPort := authority
	if at := strings.LastIndexByte(hostPort, '@'); at >= 0 {
		if !validIRIChars(hostPort[:at], ":", false) {
			return false
		}
		hostPort = hostPort[at+1:]
	}
	var port string
	if strings.HasPrefix(hostPort, "[") {
		end := strings.IndexByte(hostPort, ']')
		if end < 0 || !validIPLiteral(hostPort[1:end]) {
			return false
		}
		port = hostPort[end+1:]
	} else {
		// IPv4address and ireg-name share one character set.
		host := hostPort
		if colon := strings.IndexByte(hostPort, ':'); colon >= 0 {
			host, port = hostPort[:colon], hostPort[colon:]
		}
		if !validIRIChars(host, "", false) {
			return false
		}
	}
	if port == "" {
		return true
	}
	if port[0] != ':' {
		return false
	}
	for _, c := range port[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validIPLiteral checks the inside of IP-literal: IPv6address or IPvFuture.
func validIPLiteral(lit string) bool {
	if strings.HasPrefix(lit, "v") || strings.HasPrefix(lit, "V") {
		dot := strings.IndexByte(lit, '.')
		if dot < 2 || dot == len(lit)-1 {
			return false
		}
		for _, c := range lit[1:dot] {
			if !isHexDigit(byte(c)) || c > 0x7f {
				return false
			}
		}
		for _, c := range lit[dot+1:] {
			if c > 0x7f || !(isUnreserved(c) || isSubDelim(c) || c == ':') {
				return false
			}
		}
		return true
	}
	addr, err := netip.ParseAddr(lit)
	return err == nil && addr.Is6() && addr.Zone() == ""
}

// validIRIChars checks that s holds only iunreserved, pct-encoded and
// sub-delims characters, the characters in extra, and iprivate characters
// when private is true (only iquery allows them).
func validIRIChars(s, extra string, private bool) bool {
	for i := 0; i < len(s); {
		c, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case c == '%':
			if i+2 >= len(s) || !isHexDigit(s[i+1]) || !isHexDigit(s[i+2]) {
				return false
			}
			i += 3
			continue
		case c < 0x80:
			if !isUnreserved(c) && !isSubDelim(c) && !strings.ContainsRune(extra, c) {
				return false
			}
		case !isUCSChar(c) && !(private && isIPrivate(c)):
			return false
		}
		i += size
	}
	return true
}

func isUnreserved(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~'
}

func isSubDelim(c rune) bool {
	return strings.ContainsRune("!$&'()*+,;=", c)
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// isUCSChar is the ucschar rule of RFC 3987 §2.2.
func isUCSChar(c rune) bool {
	switch {
	case c >= 0xA0 && c <= 0xD7FF, c >= 0xF900 && c <= 0xFDCF, c >= 0xFDF0 && c <= 0xFFEF:
		return true
	case c >= 0xE0000 && c <= 0xE0FFF:
		return false
	case c >= 0x10000 && c <= 0xEFFFD:
		return c&0xFFFF <= 0xFFFD
	}
	return false
}

// isIPrivate is the iprivate rule of RFC 3987 §2.2.
func isIPrivate(c rune) bool {
	return (c >= 0xE000 && c <= 0xF8FF) || (c >= 0xF0000 && c <= 0xFFFFD) || (c >= 0x100000 && c <= 0x10FFFD)
}
