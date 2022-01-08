package presenter

import (
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/cf-k8s-controllers/api/repositories"
)

const (
	// TODO: repetition with handler endpoint?
	orgsBase    = "/v3/organizations"
	spacesBase  = "/v3/spaces"
	spacePrefix = "cfspace-"
	orgPrefix   = "cforg-"
)

type OrgResponse struct {
	Name string `json:"name"`
	GUID string `json:"guid"`

	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
	Suspended     bool          `json:"suspended"`
	Relationships Relationships `json:"relationships"`
	Metadata      Metadata      `json:"metadata"`
	Links         OrgLinks      `json:"links"`
}

type OrgLinks struct {
	Self          *Link `json:"self"`
	Domains       *Link `json:"domains,omitempty"`
	DefaultDomain *Link `json:"default_domain,omitempty"`
	Quota         *Link `json:"quota,omitempty"`
}

type SpaceResponse struct {
	Name          string        `json:"name"`
	GUID          string        `json:"guid"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
	Links         SpaceLinks    `json:"links"`
	Metadata      Metadata      `json:"metadata"`
	Relationships Relationships `json:"relationships"`
}

type SpaceLinks struct {
	Self         *Link `json:"self"`
	Organization *Link `json:"organization"`
}

func ForCreateOrg(org repositories.OrgRecord, apiBaseURL url.URL) OrgResponse {
	return toOrgResponse(org, apiBaseURL)
}

func ForOrgList(orgs []repositories.OrgRecord, apiBaseURL, requestURL url.URL) ListResponse {
	orgResponses := make([]interface{}, 0, len(orgs))
	for _, org := range orgs {
		orgResponses = append(orgResponses, toOrgResponse(org, apiBaseURL))
	}

	return ForList(orgResponses, apiBaseURL, requestURL)
}

func ForCreateSpace(space repositories.SpaceRecord, apiBaseURL url.URL) SpaceResponse {
	return toSpaceResponse(space, apiBaseURL)
}

func ForSpaceList(spaces []repositories.SpaceRecord, apiBaseURL, requestURL url.URL) ListResponse {
	spaceResponses := make([]interface{}, 0, len(spaces))
	for _, space := range spaces {
		spaceResponses = append(spaceResponses, toSpaceResponse(space, apiBaseURL))
	}

	return ForList(spaceResponses, apiBaseURL, requestURL)
}

func toSpaceResponse(space repositories.SpaceRecord, apiBaseURL url.URL) SpaceResponse {

	return SpaceResponse{
		Name:      space.Name,
		GUID:      strings.TrimPrefix(space.GUID, spacePrefix),
		CreatedAt: space.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: space.CreatedAt.UTC().Format(time.RFC3339),
		Metadata: Metadata{
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Relationships: Relationships{
			"organization": Relationship{
				Data: &RelationshipData{
					GUID: strings.TrimPrefix(space.OrganizationGUID, orgPrefix),
				},
			},
		},
		Links: SpaceLinks{
			Self: &Link{
				HREF: buildURL(apiBaseURL).appendPath(spacesBase, strings.TrimPrefix(space.GUID, spacePrefix)).build(),
			},
			Organization: &Link{
				HREF: buildURL(apiBaseURL).appendPath(orgsBase, strings.TrimPrefix(space.OrganizationGUID, orgPrefix)).build(),
			},
		},
	}
}

func toOrgResponse(org repositories.OrgRecord, apiBaseURL url.URL) OrgResponse {

	return OrgResponse{
		Name:      org.Name,
		GUID:      strings.TrimPrefix(org.GUID, orgPrefix),
		CreatedAt: org.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: org.CreatedAt.UTC().Format(time.RFC3339),
		Suspended: org.Suspended,
		Metadata: Metadata{
			Labels:      orEmptyMap(org.Labels),
			Annotations: orEmptyMap(org.Annotations),
		},
		Relationships: Relationships{},
		Links: OrgLinks{
			Self: &Link{
				HREF: buildURL(apiBaseURL).appendPath(orgsBase, strings.TrimPrefix(org.GUID, orgPrefix)).build(),
			},
		},
	}
}

func orEmptyMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
