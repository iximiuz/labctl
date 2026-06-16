package api

import (
	"context"
	"slices"
	"strings"
)

// Known regions mirror the backend's KNOWN_REGIONS (see foreman/shared/utils/geo.js).
const (
	RegionEU = "eu"
	RegionAP = "ap"
)

var KnownRegions = []string{RegionEU, RegionAP}

func IsKnownRegion(region string) bool {
	return slices.Contains(KnownRegions, region)
}

func KnownRegionsString() string {
	return strings.Join(KnownRegions, ", ")
}

// SetPreferredRegion sets the user's preferred region for new playgrounds.
func (c *Client) SetPreferredRegion(ctx context.Context, region string) (*Me, error) {
	body, err := toJSONBody(map[string]any{"region": region})
	if err != nil {
		return nil, err
	}

	var me Me
	return &me, c.PatchInto(ctx, "/account/preferences", nil, nil, body, &me)
}
