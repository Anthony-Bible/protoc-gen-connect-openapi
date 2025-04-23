package googleapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartsToOpenAPIPath(t *testing.T) {
	t.Run("with annotation", func(t *testing.T) {
		v, err := RunPathPatternLexer("/pet/{pet_id}:addPet")
		require.NoError(t, err)
		path := partsToOpenAPIPath(v)
		assert.Equal(t, "/pet/{pet_id}:addPet", path)
	})

	t.Run("with glob pattern", func(t *testing.T) {
		v, err := RunPathPatternLexer("/tcn/bi/businessIntelligence/v1alpha1/{name=orgs/*/regions/*/dashboards/*}:publish")
		require.NoError(t, err)
		path := partsToOpenAPIPath(v)
		assert.Equal(t, "/tcn/bi/businessIntelligence/v1alpha1/orgs/{org}/regions/{region}/dashboards/{dashboard}:publish", path)
	})
}
