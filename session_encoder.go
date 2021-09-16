package phpsessgo

import "github.com/eligundry/phpsessgo/phpencode"

type SessionEncoder interface {
	Encode(session phpencode.PhpSession) (string, error)
	Decode(raw string) (phpencode.PhpSession, error)
}
