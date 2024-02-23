package utils

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/anthdm/hollywood/actor"
)

func ForwardActorRequest(ctx *actor.Context, pid *actor.PID, msg any) {
	ctx.Engine().SendWithSender(pid, msg, ctx.Sender())
}

func RespondWithError(ctx *actor.Context, err error) error {
	ctx.Respond(err)
	return err
}

func HandleResponse[T any](response *actor.Response) (T, error) {
	var none T
	result, err := response.Result()
	if err != nil {
		return none, err
	}

	switch result := result.(type) {
	case T:
		return result, nil
	case error:
		return none, result
	default:
		return none, errors.New("invalid result")
	}
}

func LogActorError(ctx *actor.Context, err error) {
	if err != nil {
		log.Printf("ERROR: %s - %T - %s", ctx.PID().GetID(), ctx.Message(), err.Error())
	}
}

func LogActorMsg(ctx *actor.Context) {
	log.Printf("MESSAGE: %s - %T", ctx.PID().GetID(), ctx.Message())
}

type PoisonTimer struct {
	c chan struct{}
	mu sync.Mutex
}

func (pt *PoisonTimer) StartWithCallback(e *actor.Engine, pid *actor.PID, d time.Duration, c func() error) {
	pt.c = make(chan struct{})
	timer := time.NewTimer(d)
	go func() {
		for {
			select {
			case <-pt.c:
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(d)
			case <-timer.C:
				locked := pt.mu.TryLock()
				if !locked {
					timer.Reset(d)
					continue
				}
				log.Printf("POISON: %s", pid.GetID())
				if c != nil {
					err := c()
					if err != nil {
						log.Printf("POISON ERROR: %s - %s", pid.GetID(), err.Error())
					}
				}
				e.Poison(pid)
				pt.c = nil
				pt.mu.Unlock()
				return
			}
		}
	}()
}

func (pt *PoisonTimer) Start(e *actor.Engine, pid *actor.PID, d time.Duration) {
	pt.StartWithCallback(e, pid, d, nil)
}

func (pt *PoisonTimer) Reset() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.c != nil {
		pt.c <- struct{}{}
	}
}

