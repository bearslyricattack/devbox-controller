package registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/avast/retry-go"
)

type Client struct {
	Username string
	Password string
}

type Manifest struct {
	SchemaVersion int     `json:"schemaVersion"`
	MediaType     string  `json:"mediaType"`
	Config        Config  `json:"config"`
	Layers        []Layer `json:"layers"`
}

type Config struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int    `json:"size"`
}

type Layer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int    `json:"size"`
}

var (
	ErrorManifestNotFound = errors.New("manifest not found")
	ErrorMediaTypeInvalid = errors.New("media type invalid")
)

func (t *Client) TagImage(hostName string, imageName string, oldTag string, newTag string) error {
	return retry.Do(func() error {
		manifest, mediaType, err := t.pullManifest(t.Username, t.Password, hostName, imageName, oldTag)
		if err != nil {
			return err
		}
		return t.pushManifest(t.Username, t.Password, hostName, imageName, newTag, manifest, mediaType)
	}, retry.Delay(time.Second*5), retry.Attempts(3), retry.LastErrorOnly(true))
}

func (t *Client) login(authPath string, username string, password string, imageName string) (string, error) {
	var (
		client = http.DefaultClient
		url    = authPath + imageName + ":pull,push"
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}

	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var data struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		IssuedAt    string `json:"issued_at"`
	}
	if err := json.Unmarshal(bodyText, &data); err != nil {
		return "", err
	}
	if data.Token == "" {
		return "", errors.New("empty token")
	}
	return data.Token, nil
}

func (t *Client) pullManifest(username string, password string, hostName string, imageName string, tag string) ([]byte, string, error) {
	var (
		client = http.DefaultClient
		url    = "http://" + hostName + "/v2/" + imageName + "/manifests/" + tag
	)
	fmt.Println("访问的url为：" + url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrorManifestNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.New(resp.Status)
	}

	bodyText, err := io.ReadAll(resp.Body)

	var manifest Manifest
	err = json.Unmarshal(bodyText, &manifest)
	if err != nil {
		return nil, "", err
	}
	if len(manifest.MediaType) == 0 {
		return nil, "", ErrorMediaTypeInvalid
	}
	fmt.Println("访问的结果为：" + manifest.MediaType)
	return bodyText, manifest.MediaType, nil
}

func (t *Client) pushManifest(username string, password string, hostName string, imageName string, tag string, manifest []byte, mediaType string) error {
	var (
		client = http.DefaultClient
		url    = "http://" + hostName + "/v2/" + imageName + "/manifests/" + tag
	)
	fmt.Println("访问的url为：" + url)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(manifest))
	if err != nil {
		return err
	}

	req.SetBasicAuth(username, password)
	req.Header.Set("Content-type", mediaType)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return errors.New(resp.Status)
	}
	fmt.Println("访问的结果为：" + err.Error())
	return nil
}
