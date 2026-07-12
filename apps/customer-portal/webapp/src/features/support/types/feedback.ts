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

// A selectable chip (tag) attached to a feedback emoji.
export type FeedbackEmojiChip = {
  id: string;
  name: string;
  value: string;
};

// A feedback emoji option (rating) with its images and chips, from GET /metadata.
export type FeedbackEmoji = {
  id: string;
  name: string;
  value: string;
  unselectedImage: string;
  selectedImage: string;
  chips: FeedbackEmojiChip[];
};

// Request payload for submitting case feedback (POST /cases/{id}/feedback).
export type CaseFeedbackPayload = {
  emojiId: string;
  chipIds?: string[];
  additionalComment?: string;
};

// Submitted feedback details returned on POST.
export type SubmittedFeedback = {
  id: string;
  assessmentId: string;
  caseId: string;
  createdBy: string;
  createdOn: string;
};

// Response from submitting case feedback.
export type CaseFeedbackResponse = {
  message: string;
  feedback: SubmittedFeedback;
};

// Emoji summary attached to an existing case feedback.
export type FeedbackEmojiSummary = {
  id: string;
  name: string;
  selectedImage: string;
};

// Existing case feedback (GET /cases/{id}/feedback).
export type CaseFeedback = {
  id: string;
  emoji: FeedbackEmojiSummary;
  chips?: string[] | null;
  assessmentId: string;
  createdBy: string;
  createdOn: string;
  additionalComment?: string | null;
};
