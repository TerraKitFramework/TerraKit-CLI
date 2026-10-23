//go:build !cgo

package native

import (
	"errors"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/catalog"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/mesh"
)

var errDisabled = errors.New("TerraKit native integration requires CGO_ENABLED=1 and a C compiler")

type Library struct{}

func Open(string) (*Library, error)                { return nil, errDisabled }
func (*Library) Version() string                   { return "" }
func (*Library) Close()                            {}
func (*Library) Catalog() ([]catalog.Stage, error) { return nil, errDisabled }
func (*Library) Generate(config.Document, []catalog.Stage) (mesh.Data, error) {
	return mesh.Data{}, errDisabled
}
