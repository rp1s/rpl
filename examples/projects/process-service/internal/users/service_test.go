package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	model "example.com/rpl/process-service/generated/user"
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

func TestGeneratedHTTPClientAndHandler(t *testing.T) {
	server := httptest.NewServer(transport.NewUserHTTPHandler(NewService()))
	defer server.Close()

	client, err := transport.NewUserHTTPClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	saved, err := client.Put(context.Background(), model.User{Id: 11, Name: "Grace"})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if saved.Name != "Grace" {
		t.Fatalf("saved name = %q", saved.Name)
	}
	label, err := client.Label(context.Background(), saved.Id)
	if err != nil {
		t.Fatalf("label: %v", err)
	}
	if label != "11:Grace" {
		t.Fatalf("label = %q", label)
	}
}

func TestGeneratedUnixClientAndServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix domain sockets are not a portable Windows runtime feature")
	}
	placeholder, err := os.CreateTemp("", "rpl-users-*.sock")
	if err != nil {
		t.Fatalf("reserve socket path: %v", err)
	}
	socket := placeholder.Name()
	_ = placeholder.Close()
	_ = os.Remove(socket)
	t.Cleanup(func() { _ = os.Remove(socket) })
	server, err := transport.ListenUserUnix(socket, NewService())
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client, err := transport.DialUserUnix(context.Background(), socket)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	if _, err := client.Put(context.Background(), model.User{Id: 12, Name: "Linus"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	found, err := client.GetByID(context.Background(), 12)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.Name != "Linus" {
		t.Fatalf("found name = %q", found.Name)
	}
}
