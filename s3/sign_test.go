package s3

import (
	"bufio"
	"bytes"
	"net/http"
	"strings"
	"testing"
)

type test struct {
	Name                string
	Request             string
	CanonicalRequest    string
	StringToSign        string
	AuthorizationHeader string
}

// These tests are extracted from aws4_testsuite.zip, retrieved on 2015-04-23:
// http://docs.aws.amazon.com/general/latest/gr/signature-v4-test-suite.html
// sha1sum aws4_testsuite.zip = fd51507d9b3c422b9da8c3e12e56501a76f97ddb
var tests = []test{
	{
		Name:                "get-header-key-duplicate",
		Request:             "POST / http/1.1\r\nDATE:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\nZOO:zoobar\r\nzoo:foobar\r\nzoo:zoobar\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\nzoo:foobar,zoobar,zoobar\r\n\r\ndate;host;zoo\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n3c52f0eaae2b61329c0a332e3fa15842a37bc5812cf4d80eb64784308850e313",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host;zoo, Signature=54afcaaf45b331f81cd2edb974f7b824ff4dd594cbbaa945ed636b48477368ed",
	},
	{
		Name:                "get-header-value-order",
		Request:             "POST / http/1.1\r\nDATE:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\np:z\r\np:a\r\np:p\r\np:a\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\np:a,a,p,z\r\n\r\ndate;host;p\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n94c0389fefe0988cbbedc8606f0ca0b485b48da010d09fc844b45b697c8924fe",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host;p, Signature=d2973954263943b11624a11d1c963ca81fb274169c7868b2858c04f083199e3d",
	},
	{
		Name:                "get-header-value-trim",
		Request:             "POST / http/1.1\r\nDATE:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\np: phfft \r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\np:phfft\r\n\r\ndate;host;p\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\ndddd1902add08da1ac94782b05f9278c08dc7468db178a84f8950d93b30b1f35",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host;p, Signature=debf546796015d6f6ded8626f5ce98597c33b47b9164cf6b17b4642036fcb592",
	},
	{
		Name:                "get-relative-relative",
		Request:             "GET /foo/bar/../.. http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n366b91fb121d72a00f46bbe8d395f53a102b06dfb7e79636515208ed3fa606b1",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b27ccfbfa7df52a200ff74193ca6e32d4b48b8856fab7ebf1c595d0670a7e470",
	},
	{
		Name:                "get-relative",
		Request:             "GET /foo/.. http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n366b91fb121d72a00f46bbe8d395f53a102b06dfb7e79636515208ed3fa606b1",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b27ccfbfa7df52a200ff74193ca6e32d4b48b8856fab7ebf1c595d0670a7e470",
	},
	{
		Name:                "get-slash-dot-slash",
		Request:             "GET /./ http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n366b91fb121d72a00f46bbe8d395f53a102b06dfb7e79636515208ed3fa606b1",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b27ccfbfa7df52a200ff74193ca6e32d4b48b8856fab7ebf1c595d0670a7e470",
	},
	{
		Name:                "get-slash-pointless-dot",
		Request:             "GET /./foo http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/foo\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n8021a97572ee460f87ca67f4e8c0db763216d84715f5424a843a5312a3321e2d",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=910e4d6c9abafaf87898e1eb4c929135782ea25bb0279703146455745391e63a",
	},
	{
		Name:                "get-slash",
		Request:             "GET // http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n366b91fb121d72a00f46bbe8d395f53a102b06dfb7e79636515208ed3fa606b1",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b27ccfbfa7df52a200ff74193ca6e32d4b48b8856fab7ebf1c595d0670a7e470",
	},
	{
		Name:                "get-slashes",
		Request:             "GET //foo// http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/foo/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n6bb4476ee8745730c9cb79f33a0c70baa6d8af29c0077fa12e4e8f1dd17e7098",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b00392262853cfe3201e47ccf945601079e9b8a7f51ee4c3d9ee4f187aa9bf19",
	},
	{
		Name:                "get-space",
		Request:             "GET /%20/foo http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/%20/foo\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n69c45fb9fe3fd76442b5086e50b2e9fec8298358da957b293ef26e506fdfb54b",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=f309cfbd10197a230c42dd17dbf5cca8a0722564cb40a872d25623cfa758e374",
	},
	{
		Name:                "get-unreserved",
		Request:             "GET /-._~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/-._~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\ndf63ee3247c0356c696a3b21f8d8490b01fa9cd5bc6550ef5ef5f4636b7b8901",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=830cc36d03f0f84e6ee4953fbe701c1c8b71a0372c63af9255aa364dd183281e",
	},
	{
		Name:                "get-utf8",
		Request:             "GET /%E1%88%B4 http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/%E1%88%B4\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n27ba31df5dbc6e063d8f87d62eb07143f7f271c5330a917840586ac1c85b6f6b",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=8d6634c189aa8c75c2e51e106b6b5121bed103fdb351f7d7d4381c738823af74",
	},
	{
		Name:                "get-vanilla-empty-query-key",
		Request:             "GET /?foo=bar http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\nfoo=bar\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n0846c2945b0832deb7a463c66af5c4f8bd54ec28c438e67a214445b157c9ddf8",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=56c054473fd260c13e4e7393eb203662195f5d4a1fada5314b8b52b23f985e9f",
	},
	{
		Name:                "get-vanilla-query-order-key-case",
		Request:             "GET /?foo=Zoo&foo=aha http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\nfoo=Zoo&foo=aha\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\ne25f777ba161a0f1baf778a87faf057187cf5987f17953320e3ca399feb5f00d",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=be7148d34ebccdc6423b19085378aa0bee970bdc61d144bd1a8c48c33079ab09",
	},
	{
		Name:                "get-vanilla-query-order-key",
		Request:             "GET /?a=foo&b=foo http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\na=foo&b=foo\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n2f23d14fe13caebf6dfda346285c6d9c14f49eaca8f5ec55c627dd7404f7a727",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=0dc122f3b28b831ab48ba65cb47300de53fbe91b577fe113edac383730254a3b",
	},
	{
		Name:                "get-vanilla-query-order-value",
		Request:             "GET /?foo=b&foo=a http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\nfoo=a&foo=b\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n33dffc220e89131f8f6157a35c40903daa658608d9129ff9489e5cf5bbd9b11b",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=feb926e49e382bec75c9d7dcb2a1b6dc8aa50ca43c25d2bc51143768c0875acc",
	},
	{
		Name:                "get-vanilla-query-unreserved",
		Request:             "GET /?-._~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz=-._~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\n-._~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz=-._~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\nd2578f3156d4c9d180713d1ff20601d8a3eed0dd35447d24603d7d67414bd6b5",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=f1498ddb4d6dae767d97c466fb92f1b59a2c71ca29ac954692663f9db03426fb",
	},
	{
		Name:                "get-vanilla-query",
		Request:             "GET / http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n366b91fb121d72a00f46bbe8d395f53a102b06dfb7e79636515208ed3fa606b1",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b27ccfbfa7df52a200ff74193ca6e32d4b48b8856fab7ebf1c595d0670a7e470",
	},
	{
		Name:                "get-vanilla-ut8-query",
		Request:             "GET /?ሴ=bar http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\n%E1%88%B4=bar\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\nde5065ff39c131e6c2e2bd19cd9345a794bf3b561eab20b8d97b2093fc2a979e",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=6fb359e9a05394cc7074e0feb42573a2601abc0c869a953e8c5c12e4e01f1a8c",
	},
	{
		Name:                "get-vanilla",
		Request:             "GET / http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "GET\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n366b91fb121d72a00f46bbe8d395f53a102b06dfb7e79636515208ed3fa606b1",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b27ccfbfa7df52a200ff74193ca6e32d4b48b8856fab7ebf1c595d0670a7e470",
	},
	{
		Name:                "post-header-key-case",
		Request:             "POST / http/1.1\r\nDATE:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n05da62cee468d24ae84faff3c39f1b85540de60243c1bcaace39c0a2acc7b2c4",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=22902d79e148b64e7571c3565769328423fe276eae4b26f83afceda9e767f726",
	},
	{
		Name:                "post-header-key-sort",
		Request:             "POST / http/1.1\r\nDATE:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\nZOO:zoobar\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\nzoo:zoobar\r\n\r\ndate;host;zoo\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n34e1bddeb99e76ee01d63b5e28656111e210529efeec6cdfd46a48e4c734545d",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host;zoo, Signature=b7a95a52518abbca0964a999a880429ab734f35ebbf1235bd79a5de87756dc4a",
	},
	{
		Name:                "post-header-value-case",
		Request:             "POST / http/1.1\r\nDATE:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\nzoo:ZOOBAR\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\nzoo:ZOOBAR\r\n\r\ndate;host;zoo\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n3aae6d8274b8c03e2cc96fc7d6bda4b9bd7a0a184309344470b2c96953e124aa",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host;zoo, Signature=273313af9d0c265c531e11db70bbd653f3ba074c1009239e8559d3987039cad7",
	},
	{
		Name:                "post-vanilla-empty-query-value",
		Request:             "POST /?foo=bar http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\nfoo=bar\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\ncd4f39132d8e60bb388831d734230460872b564871c47f5de62e62d1a68dbe1e",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b6e3b79003ce0743a491606ba1035a804593b0efb1e20a11cba83f8c25a57a92",
	},
	{
		Name:                "post-vanilla-query-nonunreserved",
		Request:             "POST /?@#$%^&+=/,?><`\";:\\|][{} =@#$%^&+=/,?><`\";:\\|][{}  http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\n%20=%2F%2C%3F%3E%3C%60%22%3B%3A%5C%7C%5D%5B%7B%7D&%40%23%24%25%5E=\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\neb3f16b23b20c91e1b5d6f3cd1c1f8c32a6ddcae6024e44bcfa980fbf8561f6c",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=28675d93ac1d686ab9988d6617661da4dffe7ba848a2285cb75eac6512e861f9",
	},
	{
		Name:                "post-vanilla-query-space",
		Request:             "POST /?f oo=b ar http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\nf=\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n0d5f8ed88e9cd0a2a093e1719fff945e3718d30a6b240b9de994cdf9442c89f5",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b7eb653abe5f846e7eee4d1dba33b15419dc424aaf215d49b1240732b10cc4ca",
	},
	{
		Name:                "post-vanilla-query",
		Request:             "POST /?foo=bar http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\nfoo=bar\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\ncd4f39132d8e60bb388831d734230460872b564871c47f5de62e62d1a68dbe1e",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=b6e3b79003ce0743a491606ba1035a804593b0efb1e20a11cba83f8c25a57a92",
	},
	{
		Name:                "post-vanilla",
		Request:             "POST / http/1.1\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\n",
		CanonicalRequest:    "POST\r\n/\r\n\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ndate;host\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n05da62cee468d24ae84faff3c39f1b85540de60243c1bcaace39c0a2acc7b2c4",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=date;host, Signature=22902d79e148b64e7571c3565769328423fe276eae4b26f83afceda9e767f726",
	},
	{
		Name:                "post-x-www-form-urlencoded-parameters",
		Request:             "POST / http/1.1\r\nContent-Type:application/x-www-form-urlencoded; charset=utf8\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\nfoo=bar",
		CanonicalRequest:    "POST\r\n/\r\n\r\ncontent-type:application/x-www-form-urlencoded; charset=utf8\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ncontent-type;date;host\r\n3ba8907e7a252327488df390ed517c45b96dead033600219bdca7107d1d3f88a",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\nc4115f9e54b5cecf192b1eaa23b8e88ed8dc5391bd4fde7b3fff3d9c9fe0af1f",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=content-type;date;host, Signature=b105eb10c6d318d2294de9d49dd8b031b55e3c3fe139f2e637da70511e9e7b71",
	},
	{
		Name:                "post-x-www-form-urlencoded",
		Request:             "POST / http/1.1\r\nContent-Type:application/x-www-form-urlencoded\r\nDate:Mon, 09 Sep 2011 23:36:00 GMT\r\nHost:host.foo.com\r\n\r\nfoo=bar",
		CanonicalRequest:    "POST\r\n/\r\n\r\ncontent-type:application/x-www-form-urlencoded\r\ndate:Mon, 09 Sep 2011 23:36:00 GMT\r\nhost:host.foo.com\r\n\r\ncontent-type;date;host\r\n3ba8907e7a252327488df390ed517c45b96dead033600219bdca7107d1d3f88a",
		StringToSign:        "AWS4-HMAC-SHA256\r\n20110909T233600Z\r\n20110909/us-east-1/host/aws4_request\r\n4c5c6e4b52fb5fb947a8733982a8a5a61b14f04345cbfe6e739236c76dd48f74",
		AuthorizationHeader: "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/us-east-1/host/aws4_request, SignedHeaders=content-type;date;host, Signature=5a15b22cf462f047318703b92e6f4f38884e4a7ab7b1d6426ca46a8bd1c26cbc",
	},
}

// All examples in the test suite use the following credential scope:
// AKIDEXAMPLE/20110909/us-east-1/host/aws4_request
// The example secret key used for signing is:
// wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY
var service = &Service{
	Name:        "host",
	Region:      "us-east-1",
	AccessKey:   "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
	AccessKeyId: "AKIDEXAMPLE",
}

var ignoreTest = map[string]bool{
	"get-slashes":                           true, // unsure whether /foo// = /foo/
	"post-vanilla-query-nonunreserved":      true, // malformed HTTP version
	"post-vanilla-query-space":              true, // malformed HTTP version
	"post-x-www-form-urlencoded-parameters": true, // https://github.com/golang/go/issues/3958
	"post-x-www-form-urlencoded":            true, // https://github.com/golang/go/issues/3958
}

// The remaining tests are broken in other ways, so we have to fix them.
func fixTest(t test) test {
	return test{
		Name:                t.Name,
		Request:             strings.Replace(t.Request, "http/1.1", "HTTP/1.1", 1),
		CanonicalRequest:    strings.Replace(t.CanonicalRequest, "\r\n", "\n", -1),
		StringToSign:        strings.Replace(t.StringToSign, "\r\n", "\n", -1),
		AuthorizationHeader: t.AuthorizationHeader,
	}
}

func TestSign(t *testing.T) {
	for _, brokenTest := range tests {
		if ignoreTest[brokenTest.Name] {
			continue
		}
		test := fixTest(brokenTest)

		reader := bufio.NewReader(bytes.NewReader([]byte(test.Request)))
		req, err := http.ReadRequest(reader)
		if err != nil {
			t.Fatalf("%s: %s", test.Name, err)
		}

		buf := new(bytes.Buffer)
		canonicalRequest(buf, req)
		cr := buf.String()
		if cr != test.CanonicalRequest {
			t.Fatalf("test: %s\nexpected: %q\nactually: %q", test.Name, test.CanonicalRequest, cr)
		}

		auth := service.authorization(req)
		if auth != test.AuthorizationHeader {
			t.Fatalf("test: %s\nexpected: %s\nactually: %s", test.Name, test.AuthorizationHeader, auth)
		}
	}
}
