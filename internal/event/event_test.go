package event_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/victornm/equiz/internal/event"
)

func TestBus_PublishSubscribe(t *testing.T) {
	type (
		inputs struct {
			published   []event.Event
			subscribers []subscriber
		}

		outputs struct {
			received map[string][]event.Event
		}
	)

	tests := map[string]struct {
		arrange func() inputs
		assert  func(t *testing.T, out outputs)
	}{
		"a single subscriber should receive correct event": {
			arrange: func() inputs {
				return inputs{
					published: []event.Event{
						eventWithName("e1"),
						eventWithName("e2"),
					},
					subscribers: []subscriber{
						{
							name:        "s1",
							subscribeTo: []string{"e1"},
						},
					},
				}
			},

			assert: func(t *testing.T, out outputs) {
				assert.ElementsMatch(t, []event.Event{eventWithName("e1")}, out.received["s1"])
			},
		},

		"a single subscriber should receive all dispatched event": {
			arrange: func() inputs {
				return inputs{
					published: []event.Event{
						eventWithName("e1"),
						eventWithName("e1"),
					},
					subscribers: []subscriber{
						{
							name:        "s1",
							subscribeTo: []string{"e1"},
						},
					},
				}
			},

			assert: func(t *testing.T, out outputs) {
				assert.ElementsMatch(t, []event.Event{eventWithName("e1"), eventWithName("e1")}, out.received["s1"])
			},
		},

		"an event should be dispatched to all subscribers": {
			arrange: func() inputs {
				return inputs{
					published: []event.Event{
						eventWithName("e1"),
					},
					subscribers: []subscriber{
						{
							name:        "s1",
							subscribeTo: []string{"e1"},
						},
						{
							name:        "s2",
							subscribeTo: []string{"e1"},
						},
						{
							name:        "s3",
							subscribeTo: []string{"e1"},
						},
					},
				}
			},

			assert: func(t *testing.T, out outputs) {
				assert.ElementsMatch(t, []event.Event{eventWithName("e1")}, out.received["s1"])
				assert.ElementsMatch(t, []event.Event{eventWithName("e1")}, out.received["s2"])
				assert.ElementsMatch(t, []event.Event{eventWithName("e1")}, out.received["s3"])
			},
		},

		"multiple events should be dispatched correctly multiple subscribers": {
			arrange: func() inputs {
				return inputs{
					published: []event.Event{
						eventWithName("e1"),
						eventWithName("e2"),
						eventWithName("e1"),
						eventWithName("e3"),
					},
					subscribers: []subscriber{
						{
							name:        "s1",
							subscribeTo: []string{"e1"},
						},
						{
							name:        "s2",
							subscribeTo: []string{"e1", "e2"},
						},
						{
							name:        "s3",
							subscribeTo: []string{"e3", "e2"},
						},
					},
				}
			},

			assert: func(t *testing.T, out outputs) {
				assert.ElementsMatch(t, []event.Event{eventWithName("e1"), eventWithName("e1")}, out.received["s1"])
				assert.ElementsMatch(t, []event.Event{eventWithName("e1"), eventWithName("e1"), eventWithName("e2")}, out.received["s2"])
				assert.ElementsMatch(t, []event.Event{eventWithName("e2"), eventWithName("e3")}, out.received["s3"])
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			in := tt.arrange()
			mu := sync.Mutex{}
			out := outputs{received: make(map[string][]event.Event)}

			b := event.NewBus()
			for _, s := range in.subscribers {
				for _, e := range s.subscribeTo {
					b.Subscribe(e, func(ctx context.Context, e event.Event) error {
						mu.Lock()
						out.received[s.name] = append(out.received[s.name], e)
						mu.Unlock()
						return nil
					})
				}
			}

			for _, e := range in.published {
				b.Publish(context.Background(), e)
			}
			b.Stop()

			tt.assert(t, out)
		})
	}
}

type eventWithName string

func (e eventWithName) Name() string {
	return string(e)
}

type subscriber struct {
	name        string
	subscribeTo []string
}
