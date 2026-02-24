package sablier_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/storetest"
	"go.uber.org/mock/gomock"
)

func setupSablier(t *testing.T) (*sablier.Sablier, *storetest.MockStore, *providertest.MockProvider) {
	t.Helper()
	ctrl := gomock.NewController(t)

	p := providertest.NewMockProvider(ctrl)
	s := storetest.NewMockStore(ctrl)

	opts := &server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true}
	srv := server.New(opts)
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server did not start in time")
	}

	addr := srv.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("nats://%s:%d", opts.Host, addr.Port)

	nc, _ := nats.Connect(url)
	t.Cleanup(func() {
		nc.Close()
		srv.Shutdown()
	})

	m := sablier.New(slogt.New(t), s, p, nc)
	return m, s, p
}
