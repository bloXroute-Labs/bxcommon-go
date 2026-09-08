package message

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDefaultEliteAccountIsGraded pins the DI-4161 contract: the fallback account served when
// the SDN cannot supply a real account model must carry an explicit grade, never the zero
// value. Consumers attach policy to grade 0 (the ungraded grade), so a fallback that looked
// ungraded would hand every account that policy for the duration of an SDN outage.
func TestDefaultEliteAccountIsGraded(t *testing.T) {
	account := GetDefaultEliteAccount(time.Now().UTC())

	assert.Equal(t, DefaultAccountGrade, account.AccountInfo.BSCGrade)
	assert.Equal(t, DefaultAccountGrade, account.AccountInfo.ETHGrade)
	assert.NotZero(t, DefaultAccountGrade)
}
