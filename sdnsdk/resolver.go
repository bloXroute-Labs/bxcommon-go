package sdnsdk

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	log "github.com/bloXroute-Labs/bxcommon-go/logger"
)

func init() {
	IPResolverHolder = &PublicIPResolver{}
}

const publicIPResolver = "http://checkip.dyndns.org/"

var ipRegex, _ = regexp.Compile("[0-9]+(?:\\.[0-9]+){3}")

// IPResolverHolder
var (
	IPResolverHolder IPResolver
)

// MockIPResolver is the mock ip resolver
type MockIPResolver struct {
	IP string
}

// GetPublicIP is the mock ip resolver's `GetPublicIP` func
func (m *MockIPResolver) GetPublicIP() (string, error) {
	return m.IP, nil
}

// IPResolver represents an interface
type IPResolver interface {
	GetPublicIP() (string, error)
}

// PublicIPResolver represents ip resolver struct
type PublicIPResolver struct{}

// GetPublicIP fetches the publicly seen IP address of the currently running process.
func (*PublicIPResolver) GetPublicIP() (string, error) {
	response, err := http.Get(publicIPResolver)
	if err != nil {
		return "", err
	}

	defer func() {
		err = response.Body.Close()
		if err != nil {
			log.Error(fmt.Errorf("unable to close response body %v error %v", response.Body, err))
		}
	}()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != 200 {
		return "", errors.New(string(body))
	}

	return string(ipRegex.Find(body)), nil
}
