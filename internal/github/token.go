package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func createJWTToken(appID, appPrivateKey string) (string, error) {
	now := time.Now()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": jwt.NewNumericDate(now),
		"exp": jwt.NewNumericDate(now.Add(5 * time.Minute)),
		"iss": appID,
	})

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(appPrivateKey))
	if err != nil {
		return "", err
	}

	jwtToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", err
	}

	return jwtToken, nil
}

func (c client) createBearerToken() (string, error) {
	url := fmt.Sprintf("%v/app/installations/%v/access_tokens", c.apiURL, c.appInstallationID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}

	jwtToken, err := createJWTToken(c.appID, c.appPrivateKey)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", "Bearer "+jwtToken)
	req.Header.Add("Content-Type", "application/vnd.github+json")

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("error getting bearer token, got %v: %v", resp.StatusCode, string(body))
	}

	var bearer map[string]interface{}
	if err := json.Unmarshal(body, &bearer); err != nil {
		return "", err
	}

	return bearer["token"].(string), nil
}
