// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package aws

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/budgets"
)

// budgetsRegion is where the global AWS Budgets API is served.
const budgetsRegion = "us-east-1"

// BudgetTagFetcher reads the tags of an AWS Budget by account id and budget name.
type BudgetTagFetcher interface {
	Tags(ctx context.Context, accountID, budgetName string) (map[string]string, error)
}

type budgetTagger struct {
	client *budgets.Client
}

// NewBudgetTagger returns a BudgetTagFetcher on the default AWS credential chain, or nil when no AWS config can be loaded.
func NewBudgetTagger(ctx context.Context) BudgetTagFetcher {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(budgetsRegion))
	if err != nil {
		return nil
	}
	return &budgetTagger{client: budgets.NewFromConfig(cfg)}
}

func (t *budgetTagger) Tags(ctx context.Context, accountID, budgetName string) (map[string]string, error) {
	arn := fmt.Sprintf("arn:aws:budgets::%s:budget/%s", accountID, budgetName)
	out, err := t.client.ListTagsForResource(ctx, &budgets.ListTagsForResourceInput{ResourceARN: &arn})
	if err != nil {
		return nil, err
	}
	tags := make(map[string]string, len(out.ResourceTags))
	for _, tag := range out.ResourceTags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}
	return tags, nil
}
