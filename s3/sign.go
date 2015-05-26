package s3

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Parts of this code were inpired by:
// https://github.com/bmizerany/aws4/blob/master/sign.go

// Should we consider using https://github.com/awslabs/aws-sdk-go instead?

const iso8601 = "20060102T150405Z0700"

func (s *Service) sign(r *http.Request) {
	if r.Header.Get("x-amz-date") == "" && r.Header.Get("Date") == "" {
		now := time.Now().UTC()
		r.Header.Set("x-amz-date", now.Format(iso8601))
	}
	if r.Header.Get("x-amz-content-sha256") == "" {
		r.Header.Set("x-amz-content-sha256", bodyHash(r))
	}

	auth := s.authorization(r)
	r.Header.Set("Authorization", auth)
}

func (s *Service) authorization(r *http.Request) string {
	var err error
	var now time.Time

	if date := r.Header.Get("x-amz-date"); date != "" {
		if now, err = time.Parse(iso8601, date); err != nil {
			panic(err)
		}
	} else if date := r.Header.Get("Date"); date != "" {
		if now, err = time.Parse(http.TimeFormat, date); err != nil {
			panic(err)
		}
	} else {
		panic("expecting Date or x-amx-date header in request")
	}

	h := hmac.New(sha256.New, s.signingKey(now))
	signedHeaders := s.stringToSign(h, r, now)
	shs := strings.Join(signedHeaders, ";")

	afmt := "AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%x"
	auth := fmt.Sprintf(afmt, s.AccessKeyId, s.scope(now), shs, h.Sum(nil))

	return auth
}

// credential scope
func (s *Service) scope(now time.Time) string {
	return fmt.Sprintf("%s/%s/%s/aws4_request", now.Format("20060102"), s.Region, s.Name)
}

func (s *Service) signingKey(now time.Time) []byte {
	var k []byte
	k = hmacSum([]byte("AWS4"+s.AccessKey), now.Format("20060102"))
	k = hmacSum(k, s.Region)
	k = hmacSum(k, s.Name)
	k = hmacSum(k, "aws4_request")
	return k
}

func (s *Service) stringToSign(w io.Writer, r *http.Request, now time.Time) []string {
	io.WriteString(w, "AWS4-HMAC-SHA256\n")

	io.WriteString(w, now.Format(iso8601))
	w.Write([]byte{'\n'})

	io.WriteString(w, s.scope(now))
	w.Write([]byte{'\n'})

	h := sha256.New()
	signedHeaders := canonicalRequest(h, r)
	fmt.Fprintf(w, "%x", h.Sum(nil))

	return signedHeaders
}

func canonicalRequest(w io.Writer, r *http.Request) []string {
	io.WriteString(w, r.Method)
	w.Write([]byte{'\n'})

	u := r.URL.ResolveReference(r.URL)
	io.WriteString(w, uriEncode(u.Path, false))
	w.Write([]byte{'\n'})

	canonicalQueryString(w, r.URL.Query())
	w.Write([]byte{'\n'})

	signedHeaders := canonicalHeaders(w, r)
	w.Write([]byte{'\n'})

	// HashedPayload
	hash := r.Header.Get("x-amz-content-sha256")
	if hash == "" {
		hash = bodyHash(r)
	}
	io.WriteString(w, hash)

	return signedHeaders
}

func canonicalQueryString(w io.Writer, m url.Values) {
	qs := make([]string, 0, len(m))
	for k, vs := range m {
		for _, v := range vs {
			q := uriEncode(k, true) + "=" + uriEncode(v, true)
			qs = append(qs, q)
		}
	}
	sort.Strings(qs)

	for i, q := range qs {
		if i > 0 {
			w.Write([]byte{'&'})
		}
		w.Write([]byte(q))
	}
}

func canonicalHeaders(w io.Writer, r *http.Request) []string {
	m := make(map[string]string)
	ks := make([]string, 1, len(r.Header)+1)

	m["host"] = r.Host
	ks[0] = "host"
	for h, vs := range r.Header {
		k := strings.ToLower(h)
		sort.Strings(vs)
		m[k] = strings.Join(vs, ",")
		ks = append(ks, k)
	}
	sort.Strings(ks)

	for _, k := range ks {
		io.WriteString(w, k)
		w.Write([]byte{':'})

		io.WriteString(w, m[k])
		w.Write([]byte{'\n'})
	}
	w.Write([]byte{'\n'})

	// SignedHeaders
	io.WriteString(w, strings.Join(ks, ";"))
	return ks
}

func hmacSum(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	io.WriteString(h, data)
	return h.Sum(nil)
}

func uriEncode(in string, encodeSlash bool) string {
	s := []byte(in)
	b := new(bytes.Buffer)
	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '_', c == '-', c == '~', c == '.', c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(b, "%%%X", c)
		}
	}
	return b.String()
}

func bodyHash(r *http.Request) string {
	var body []byte
	if r.Body != nil {
		var err error
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		r.Body = ioutil.NopCloser(bytes.NewReader(body))
	}
	return fmt.Sprintf("%x", sha256.Sum256(body))
}
