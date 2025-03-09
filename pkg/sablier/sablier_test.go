package sablier_test

import (
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/storetest"
	"go.uber.org/mock/gomock"
	"testing"
)

func setupSablier(t *testing.T) (sablier.Sablier, *storetest.MockStore, *providertest.MockProvider) {
	t.Helper()
	ctrl := gomock.NewController(t)

	p := providertest.NewMockProvider(ctrl)
	s := storetest.NewMockStore(ctrl)

	m := sablier.New(slogt.New(t), s, p)
	return m, s, p
}
