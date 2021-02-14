package page

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/jfyne/live"
)

// RegisterHandler the first part of the component lifecycle, this is called during component creation
// and is used to register any events that the component handles.
type RegisterHandler func(c *Component) error

// MountHandler the components mount function called on first GET request and again when the socket connects.
type MountHandler func(ctx context.Context, c *Component, r *http.Request) error

// RenderHandler ths component.
type RenderHandler func(w io.Writer, c *Component) error

// EventHandler for a component, only needs the params as the event is scoped to both the socket and then component
// iteslef. Returns any component state that needs updating.
type EventHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// ComponentConstructor a func for creating a new component.
type ComponentConstructor func(ctx context.Context, h *live.Handler, r *http.Request, s *live.Socket) (Component, error)

// Component a self contained component on the page.
type Component struct {
	// ID identifies the component on the page. This should be something stable, so that during the mount
	// it can be found again by the socket.
	ID string

	// Handler a reference to the host handler.
	Handler *live.Handler

	// Socket a reference to the socket that this component
	// is scoped too.
	Socket *live.Socket

	// Register the component. This should be used to setup event handling.
	Register RegisterHandler

	// Mount the component, this should be used to setup the components initial state.
	Mount MountHandler

	// Render the component, this should be used to describe how to render the component.
	Render RenderHandler

	// State the components state.
	State interface{}
}

// NewComponent creates a new component and returns it. It does not register it or mount it.
func NewComponent(ID string, h *live.Handler, s *live.Socket, configurations ...ComponentConfig) (Component, error) {
	c := Component{
		ID:       ID,
		Handler:  h,
		Socket:   s,
		Register: defaultRegister,
		Mount:    defaultMount,
		Render:   defaultRender,
	}
	for _, conf := range configurations {
		if err := conf(&c); err != nil {
			return Component{}, err
		}
	}

	return c, nil
}

// Init takes a constructor and then registers and mounts the component.
func Init(ctx context.Context, construct func() (Component, error)) (Component, error) {
	comp, err := construct()
	if err != nil {
		return Component{}, fmt.Errorf("could not install component on construct: %w", err)
	}
	if err := comp.Register(&comp); err != nil {
		return Component{}, fmt.Errorf("could not install component on register: %w", err)
	}
	if err := comp.Mount(ctx, &comp, nil); err != nil {
		return Component{}, fmt.Errorf("could not install component on mount: %w", err)
	}
	return comp, nil
}

// Self sends an event scoped not only to this socket, but to this specific component instance. Or any
// components sharing the same ID.
func (c *Component) Self(ctx context.Context, s *live.Socket, event live.Event) {
	event.T = c.Event(event.T)
	s.Self(ctx, event)
}

// HandleSelf handles scoped incoming events send by a components Self function.
func (c *Component) HandleSelf(event string, handler EventHandler) {
	c.Handler.HandleSelf(c.Event(event), func(ctx context.Context, s *live.Socket, p map[string]interface{}) (interface{}, error) {
		state, err := handler(ctx, p)
		if err != nil {
			return s.Assigns(), err
		}
		c.State = state
		return s.Assigns(), nil
	})
}

// HandleEvent handles a component event sent from a connected socket.
func (c *Component) HandleEvent(event string, handler EventHandler) {
	c.Handler.HandleEvent(c.Event(event), func(ctx context.Context, s *live.Socket, p map[string]interface{}) (interface{}, error) {
		state, err := handler(ctx, p)
		if err != nil {
			return s.Assigns(), err
		}
		c.State = state
		return s.Assigns(), nil
	})
}

// HandleParams handles parameter changes. Caution these handlers are not scoped to a specific component.
func (c *Component) HandleParams(handler EventHandler) {
	c.Handler.HandleParams(func(ctx context.Context, s *live.Socket, p map[string]interface{}) (interface{}, error) {
		state, err := handler(ctx, p)
		if err != nil {
			return s.Assigns(), err
		}
		c.State = state
		return s.Assigns(), nil
	})
}

// Event scopes an event string so that it applies to this instance of this component
// only.
func (c *Component) Event(event string) string {
	return c.Socket.Session.ID + "--" + c.ID + "--" + event
}

// defaultRegister is the default register handler which does nothing.
func defaultRegister(c *Component) error {
	return nil
}

// defaultMount is the default mount handler which does nothing.
func defaultMount(ctx context.Context, c *Component, r *http.Request) error {
	return nil
}

// defaultRender is the default render handler which does nothing.
func defaultRender(w io.Writer, c *Component) error {
	_, err := w.Write([]byte(fmt.Sprintf("%+v", c.State)))
	return err
}

var _ RegisterHandler = defaultRegister
var _ MountHandler = defaultMount
var _ RenderHandler = defaultRender
