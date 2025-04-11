package sdnsdk

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"time"
)

const defaultBypass = time.Second * 10

// UpdateCacheFile - update a cache file
func UpdateCacheFile(dataDir string, fileName string, value []byte) error {
	cacheFileName := path.Join(dataDir, fileName)
	f, err := os.OpenFile(cacheFileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	writer := bufio.NewWriter(f)
	_, err = writer.Write(value)
	if err != nil {
		return err
	}

	return writer.Flush()
}

// LoadCacheFile - load a cache file
func LoadCacheFile(dataDir string, fileName string) ([]byte, error) {
	cacheFileName := path.Join(dataDir, fileName)
	f, err := os.Open(cacheFileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(bufio.NewReader(f))
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
