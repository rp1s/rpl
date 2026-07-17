package cache

import (
	model "example.com/rpl/session-cache/generated/session"
	"example.com/rpl/session-cache/generated/session/validation"
)

type Entry struct {
	Key        string
	TTLSeconds int
	Values     map[string]string
}

func Encode(session model.Session) (Entry, error) {
	if err := validation.Validate(session); err != nil {
		return Entry{}, err
	}
	values, err := session.RedisHash()
	if err != nil {
		return Entry{}, err
	}
	return Entry{Key: session.RedisKey(), TTLSeconds: 3600, Values: values}, nil
}

func Decode(values map[string]string) (model.Session, error) {
	var session model.Session
	if err := session.ApplyRedisHash(values); err != nil {
		return model.Session{}, err
	}
	return session, validation.Validate(session)
}
