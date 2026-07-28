package platform

import "github.com/hibiken/asynq"

func NewAsynqClient(redisURL string) (*asynq.Client, error) {
	option, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, err
	}
	return asynq.NewClient(option), nil
}

func NewAsynqServer(redisURL string, concurrency int) (*asynq.Server, error) {
	option, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, err
	}
	return asynq.NewServer(option, asynq.Config{Concurrency: concurrency}), nil
}
