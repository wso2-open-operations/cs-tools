// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package updates

import "strconv"

// mapRecommendedUpdateLevels converts the upstream snake-case response to the
// portal camelCase response shape, mirroring processRecommendedUpdateLevels in utils.bal.
func mapRecommendedUpdateLevels(src []upstreamRecommendedUpdateLevel) []RecommendedUpdateLevel {
	out := make([]RecommendedUpdateLevel, len(src))
	for i, s := range src {
		out[i] = RecommendedUpdateLevel{
			ProductName:                   s.ProductName,
			ProductBaseVersion:            s.ProductBaseVersion,
			Channel:                       s.Channel,
			StartingUpdateLevel:           s.StartingUpdateLevel,
			EndingUpdateLevel:             s.EndingUpdateLevel,
			InstalledUpdatesCount:         s.InstalledUpdatesCount,
			InstalledSecurityUpdatesCount: s.InstalledSecurityUpdatesCount,
			Timestamp:                     s.Timestamp,
			RecommendedUpdateLevel:        s.RecommendedUpdateLevel,
			AvailableUpdatesCount:         s.AvailableUpdatesCount,
			AvailableSecurityUpdatesCount: s.AvailableSecurityUpdatesCount,
		}
	}
	return out
}

// mapProductUpdateLevels converts the upstream snake-case response to the
// portal camelCase response shape, mirroring processProductUpdateLevels in utils.bal.
func mapProductUpdateLevels(src []upstreamProductUpdateLevel) []ProductUpdateLevel {
	out := make([]ProductUpdateLevel, len(src))
	for i, s := range src {
		levels := make([]UpdateLevel, len(s.ProductUpdateLevels))
		for j, ul := range s.ProductUpdateLevels {
			levels[j] = UpdateLevel{
				ProductBaseVersion: ul.ProductBaseVersion,
				Channel:            ul.Channel,
				UpdateLevels:       ul.UpdateLevels,
			}
		}
		out[i] = ProductUpdateLevel{
			ProductName:         s.ProductName,
			ProductUpdateLevels: levels,
		}
	}
	return out
}

// buildSearchRequest constructs the upstream snake-case request from the portal
// camelCase payload and the authenticated user's email, mirroring the request
// construction in processSearchUpdatesBetweenUpdateLevels in utils.bal.
func buildSearchRequest(payload SearchPayload, userEmail string) upstreamUpdateDescriptionRequest {
	return upstreamUpdateDescriptionRequest{
		ProductName:         payload.ProductName,
		ProductVersion:      payload.ProductVersion,
		Channel:             channel,
		StartingUpdateLevel: payload.StartingUpdateLevel,
		EndingUpdateLevel:   payload.EndingUpdateLevel,
		UserEmail:           userEmail,
	}
}

// groupByUpdateLevel groups update descriptions by their update level and
// determines the overall update type for each group, mirroring groupByUpdateLevel
// and processSearchUpdatesBetweenUpdateLevels in utils.bal.
func groupByUpdateLevel(src []upstreamUpdateDescription) map[string]UpdateLevelGroup {
	groups := make(map[string]UpdateLevelGroup)

	for _, d := range src {
		key := strconv.Itoa(d.UpdateLevel)

		group, exists := groups[key]
		if !exists {
			updateType := updateTypeRegular
			if d.UpdateType == updateTypeSecurity {
				updateType = updateTypeSecurity
			}
			group = UpdateLevelGroup{UpdateType: updateType}
		}

		if d.UpdateType == updateTypeSecurity {
			group.UpdateType = updateTypeSecurity
		}

		group.UpdateDescriptionLevels = append(group.UpdateDescriptionLevels, mapUpdateDescription(d))
		groups[key] = group
	}

	return groups
}

func mapUpdateDescription(d upstreamUpdateDescription) UpdateDescription {
	advisories := make([]SecurityAdvisory, len(d.SecurityAdvisories))
	for i, a := range d.SecurityAdvisories {
		advisories[i] = SecurityAdvisory{
			ID:          a.ID,
			Overview:    a.Overview,
			Severity:    a.Severity,
			Description: a.Description,
			Impact:      a.Impact,
			Solution:    a.Solution,
			Notes:       a.Notes,
			Credits:     a.Credits,
		}
	}

	var releases []DependantRelease
	if len(d.DependantReleases) > 0 {
		releases = make([]DependantRelease, len(d.DependantReleases))
		for i, r := range d.DependantReleases {
			releases[i] = DependantRelease{
				Repository:     r.Repository,
				ReleaseVersion: r.ReleaseVersion,
			}
		}
	}

	return UpdateDescription{
		UpdateLevel:        d.UpdateLevel,
		ProductName:        d.ProductName,
		ProductVersion:     d.ProductVersion,
		Channel:            d.Channel,
		UpdateType:         d.UpdateType,
		UpdateNumber:       d.UpdateNumber,
		Description:        d.Description,
		Instructions:       d.Instructions,
		BugFixes:           d.BugFixes,
		FilesAdded:         d.FilesAdded,
		FilesModified:      d.FilesModified,
		FilesRemoved:       d.FilesRemoved,
		BundlesInfoChanges: d.BundlesInfoChanges,
		DependantReleases:  releases,
		Timestamp:          d.Timestamp,
		SecurityAdvisories: advisories,
	}
}
