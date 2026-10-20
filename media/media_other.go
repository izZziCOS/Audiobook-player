//go:build !windows && !darwin

package media

type Controls struct{}

func Start(handle func(Event)) (*Controls, error) {
    return &Controls{}, nil
}

func (c *Controls) SetTrack(track Track) error {
    return nil
}

func (c *Controls) SetStatus(status Status) error {
    return nil
}

func (c *Controls) Close() {}
