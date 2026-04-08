package sdnsdk

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strings"

	"github.com/bloXroute-Labs/bxcommon-go/types"
)

var ErrMalformedAuthHeader = errors.New("auth header is not in the correct format")

// UpdateCacheFile - update a cache file
func UpdateCacheFile(dataDir string, fileName string, value []byte) error {
	cacheFileName := path.Join(dataDir, fileName)
	return os.WriteFile(cacheFileName, value, 0644)
}

// LoadCacheFile - load a cache file
func LoadCacheFile(dataDir string, fileName string) ([]byte, error) {
	return os.ReadFile(path.Join(dataDir, fileName))
}

// GetIP checks the existence of and returns the IP address for a host name
func GetIP(host string) (string, error) {
	addr := net.ParseIP(host)
	if addr == nil {
		// If domain name provided instead of IP, convert it to an IP address
		ips, err := net.LookupHost(host)
		if err != nil {
			return "", fmt.Errorf("host provided %s is not valid - %v", host, err)
		}
		if len(ips) == 0 {
			return "", fmt.Errorf("host provided %s has no IPs behind the domain name", host)
		}

		_, err = net.LookupIP(ips[0])
		if err != nil {
			return "", fmt.Errorf("host provided %s is not valid - %v", host, err)
		}

		return ips[0], nil
	}
	return host, nil
}

// GetAccountIDSecretHashFromHeader extracts accountID and secret values from an authorization header
func GetAccountIDSecretHashFromHeader(authHeader string) (types.AccountID, string, error) {
	payload, err := base64.StdEncoding.DecodeString(authHeader)
	if err != nil {
		return "", "", fmt.Errorf("auth header is not base64 encoded: %w", err)
	}
	accountIDAndHash := strings.SplitN(string(payload), ":", 2)
	if len(accountIDAndHash) <= 1 {
		return "", "", ErrMalformedAuthHeader
	}
	accountID := types.AccountID(accountIDAndHash[0])
	secretHash := accountIDAndHash[1]
	return accountID, secretHash, nil
}
