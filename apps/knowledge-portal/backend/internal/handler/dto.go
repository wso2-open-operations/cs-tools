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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
)

// articleDTO is the public article shape. It is a whitelist: a field is served
// only because this struct names it. Author, reviewer, source case, rejection
// comment, version lineage and workflow state are not here and never will be by
// accident.
type articleDTO struct {
	ID              string     `json:"id"`
	KnowledgeBaseID string     `json:"knowledgeBaseId"`
	Title           string     `json:"title"`
	Body            string     `json:"body"`
	Number          *string    `json:"number,omitempty"`
	CreatedOn       time.Time  `json:"createdOn"`
	UpdatedOn       time.Time  `json:"updatedOn"`
	PublishedOn     *time.Time `json:"publishedOn"`
}

func toArticleDTO(a entity.KBArticle) articleDTO {
	return articleDTO{
		ID:              a.ID,
		KnowledgeBaseID: a.KnowledgeBaseID,
		Title:           a.Title,
		Body:            a.Body,
		Number:          a.Number,
		CreatedOn:       a.CreatedOn,
		UpdatedOn:       a.UpdatedOn,
		PublishedOn:     a.PublishedOn,
	}
}

type searchResponseDTO struct {
	Articles []articleDTO `json:"articles"`
	Total    int          `json:"total"`
	Limit    int          `json:"limit"`
	Offset   int          `json:"offset"`
	HasMore  bool         `json:"hasMore"`
}

// knowledgeBaseDTO is the public knowledge base shape: id and title only.
// Inactive knowledge bases are not listed.
type knowledgeBaseDTO struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type listKnowledgeBasesDTO struct {
	KnowledgeBases []knowledgeBaseDTO `json:"knowledgeBases"`
}

type relatedArticleDTO struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

type relatedArticlesDTO struct {
	Articles []relatedArticleDTO `json:"articles"`
}
