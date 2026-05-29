package cmd

import (
	"net/http"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/paths"
)

func httpDo(method, url string) (*http.Response, error) {
	return httpDoWithToken(method, url, "")
}

func httpDoWithToken(method, url, token string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

func registryForCLIPath(home string) (paths.Paths, error) {
	return paths.Resolve(home)
}
