package s3

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Service struct {
	Name        string       // "s3", "iam", ...
	Endpoint    string       // "s3.amazonaws.com"
	Region      string       // "us-east-1"
	AccessKey   string       // Secret Access Key
	AccessKeyId string       // Access Key Id
	Client      *http.Client `json:"-"`
}

type Bucket struct {
	Service *Service
	Name    string `json:"Bucket"`
}

func NewBucketFromFile(path string) (*Bucket, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b := &Bucket{
		Service: &Service{
			Name:     "s3",
			Endpoint: "s3.amazonaws.com",
		},
	}
	if err = json.Unmarshal(data, b); err != nil {
		return nil, fmt.Errorf("json decoding error: %s", err)
	}
	// TODO check for missing fields?
	return b, nil
}

func (b *Bucket) URL(path string, values url.Values) *url.URL {
	return &url.URL{
		Scheme:   "https",
		Host:     b.Name + "." + b.Service.Endpoint,
		Path:     "/" + path,
		RawQuery: values.Encode(),
	}
}

func (b *Bucket) Put(key string, data []byte) error {
	req, _ := http.NewRequest("PUT", b.URL(key, nil).String(), bytes.NewBuffer(data))
	req.Header.Set("x-amz-content-sha256", fmt.Sprintf("%x", sha256.Sum256(data)))
	b.Service.sign(req)

	resp, err := b.Service.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}

	etag := fmt.Sprintf(`"%x"`, md5.Sum(data))
	if resp.Header.Get("ETag") != etag {
		return fmt.Errorf("ETag mismatch")
	}

	return nil
}

func (b *Bucket) Get(key string) ([]byte, error) {
	req, err := http.NewRequest("GET", b.URL(key, nil).String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-amz-content-sha256", fmt.Sprintf("%x", sha256.Sum256(nil)))
	b.Service.sign(req)

	resp, err := b.Service.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

type ListBucketResult struct {
	IsTruncated    bool
	Contents       []string `xml:">Key"`
	CommonPrefixes []string `xml:">Prefix"`
}

func (b *Bucket) List(prefix, delimiter string) (*ListBucketResult, error) {
	vals := url.Values{}
	vals.Set("prefix", prefix)
	if delimiter != "" {
		vals.Set("delimiter", delimiter)
	}
	req, err := http.NewRequest("GET", b.URL("", vals).String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-amz-content-sha256", fmt.Sprintf("%x", sha256.Sum256(nil)))
	b.Service.sign(req)

	resp, err := b.Service.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}

	list := new(ListBucketResult)
	err = xml.NewDecoder(resp.Body).Decode(list)
	if err != nil {
		return nil, fmt.Errorf("unable to decode response body: %s", err)
	}

	return list, nil
}

type Delete struct {
	Keys []deleteKey `xml:"Object"`
}

type deleteKey struct {
	Key string
}

type DeleteResult struct {
	Object []string `xml:">Key"`
	Error  []*DeleteError
}

func (d *DeleteResult) GetError() error {
	if len(d.Error) == 0 {
		return nil
	}
	e := d.Error[0]
	s := fmt.Sprintf("deleting keys: %q: %s: %s", e.Key, e.Code, e.Message)
	if len(d.Error) > 1 {
		s += fmt.Sprintf(" (+%d more errors)", len(d.Error)-1)
	}
	return errors.New(s)
}

type DeleteError struct {
	Key     string
	Code    string
	Message string
}

func (b *Bucket) Delete(keys []string) (*DeleteResult, error) {
	del := Delete{Keys: make([]deleteKey, len(keys))}
	for i := range keys {
		del.Keys[i].Key = keys[i]
	}

	data, err := xml.Marshal(del)
	if err != nil {
		return nil, err
	}

	vals := url.Values{}
	vals.Set("delete", "")
	body := bytes.NewReader(data)
	req, err := http.NewRequest("POST", b.URL("", vals).String(), body)
	if err != nil {
		return nil, err
	}
	sum := md5.Sum(data)
	req.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(sum[:]))
	req.Header.Set("x-amz-content-sha256", fmt.Sprintf("%x", sha256.Sum256(data)))
	b.Service.sign(req)

	resp, err := b.Service.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}

	result := new(DeleteResult)
	if err = xml.NewDecoder(resp.Body).Decode(result); err != nil {
		return nil, fmt.Errorf("unable to decode response body: %s", err)
	}

	return result, nil
}

type ServiceError struct {
	Code      string
	Message   string
	Resource  string
	RequestId string
}

func (e *ServiceError) Error() string {
	s := fmt.Sprintf("%s (S3 error)", e.Code)
	if e.Message != "" {
		s += ": " + e.Message
	}
	return s
}

func (s *Service) client() *http.Client {
	if s.Client == nil {
		return http.DefaultClient
	}
	return s.Client
}

func (s *Service) do(req *http.Request) (*http.Response, error) {
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 300 {
		return resp, nil
	} else {
		e := new(ServiceError)
		err := xml.NewDecoder(resp.Body).Decode(e)
		resp.Body.Close()
		if err != nil {
			return resp, fmt.Errorf("unable to decode response body: %s", err)
		}
		return resp, e
	}
}
