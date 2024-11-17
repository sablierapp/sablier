package app

func (s *Sablier) StopInstance() {
	// When an instance stops, remove it from promises
	s.pubsub.Subscribe()

}
