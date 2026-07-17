package users

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	transport "example.com/rpl/process-service/generated/user/transport"
)

func TestGeneratedTransportDispatchesRequests(t *testing.T) {
	input := strings.Join([]string{
		`{"method":"Put","payload":{"user":{"Id":7,"Name":"Ada"}}}`,
		`{"method":"GetByID","payload":{"id":7}}`,
		`{"method":"Label","payload":{"id":7}}`,
	}, "\n")

	var output bytes.Buffer
	server := transport.NewUserTransportServer(NewService())
	if err := server.Serve(strings.NewReader(input), &output); err != nil {
		t.Fatalf("serve: %v", err)
	}

	decoder := json.NewDecoder(&output)
	for index := 0; index < 3; index++ {
		var response struct {
			Result json.RawMessage `json:"result"`
			Error  string          `json:"error"`
		}
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("decode response %d: %v", index, err)
		}
		if response.Error != "" {
			t.Fatalf("response %d error: %s", index, response.Error)
		}
	}
}
