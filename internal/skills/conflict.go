package skills

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/abigotado/redmine-cli/internal/errx"
)

func newQuarantineName(base string) (string, error) {
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf(".%s.redmine-cli-remove-%s", base, hex.EncodeToString(nonce[:])), nil
}

func changedFile(path, detail string) error {
	return &errx.Error{
		Code:    errx.CodeConflict,
		Reason:  "DEST_CHANGED",
		Message: fmt.Sprintf("%s; preserved at %s", detail, path),
		Hint:    "inspect the preserved file and retry only after concurrent changes stop",
	}
}
