package dashboard

import (
	"context"
	"time"
)

type StatisticWeComClient struct {
	client *RoomWelcomeWeComClient
}

func NewStatisticWeComClient(baseURL string) *StatisticWeComClient {
	return &StatisticWeComClient{client: NewRoomWelcomeWeComClient(baseURL)}
}

func (c *StatisticWeComClient) UserBehavior(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, start time.Time, end time.Time) ([]StatisticBehaviorData, error) {
	if c == nil || c.client == nil || len(userIDs) == 0 {
		return []StatisticBehaviorData{}, nil
	}
	token, err := c.client.accessToken(ctx, credential)
	if err != nil {
		return nil, err
	}
	var response struct {
		weComBaseResponse
		BehaviorData []StatisticBehaviorData `json:"behavior_data"`
	}
	err = c.client.postJSON(ctx, "cgi-bin/externalcontact/get_user_behavior_data", token, map[string]any{
		"userid":     userIDs,
		"start_time": start.Unix(),
		"end_time":   end.Unix(),
	}, &response)
	if err != nil {
		return nil, err
	}
	return response.BehaviorData, nil
}
