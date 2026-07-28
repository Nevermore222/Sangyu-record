package platform

import "github.com/hibiken/asynq"

func NewAsynqClient(redisURL string) (*asynq.Client, error) {
	option, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, err
	}
	return asynq.NewClient(option), nil
}
