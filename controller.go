package kitsync

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/knusbaum/kitsync/journal"
)

type evtype int

const (
	ev_NONE evtype = iota
	ev_put
	ev_add_tag
)

type event struct {
	T    evtype
	ID   string
	Tagk string
	Tagv string
}

type Controller struct {
	primary     Storage
	secondaries []Storage
	journal     *journal.Journal[event]

	period   time.Duration
	sdo      sync.Once
	shutdown chan struct{}

	plock sync.RWMutex // lock on the primary storage
	slock sync.RWMutex // lock on the secondary storage
	wg    sync.WaitGroup
}

// NewController creates a new controller for the primary Storage. Local data such as the journal
// is stored in the directory at dir.
func NewController(dir string, primary Storage) (*Controller, error) {
	c := &Controller{
		primary:  primary,
		period:   10 * time.Second,
		shutdown: make(chan struct{}),
	}
	jpath := path.Join(dir, "journal")
	if _, err := os.Stat(jpath); os.IsNotExist(err) {
		j, err := journal.NewJournal[event](jpath, 2_000_000, 10*time.Second)
		if err != nil {
			return nil, err
		}
		c.journal = j
	} else {
		j, err := journal.OpenJournal[event](jpath)
		if err != nil {
			return nil, err
		}
		c.journal = j
	}
	log.Printf("Controller for storage %s at checkpoint %d, with journal at checkpoint %d", primary, primary.Checkpoint(), c.journal.Latest())

	if c.primary.Checkpoint() != c.journal.Latest() {
		log.Printf("Journal and Primary Storage out of sync. Doing a full resync.\n")
		err := c.Sync()
		if err != nil {
			return nil, err
		}
	}
	go c.worker()
	return c, nil
}

func (c *Controller) AddSecondary(s Storage) {
	c.slock.Lock()
	defer c.slock.Unlock()
	c.secondaries = append(c.secondaries, s)
	log.Printf("Added secondary %s at checkpoint %d\n", s, s.Checkpoint())
}

func copyObject(dst, src Storage, secondaries []Storage, id string) error {
	o, ok := src.Get(id)
	if !ok {
		return fmt.Errorf("Failed to retrieve %v from %#v\n", id, src)
	}
	// value in s not present in c.primary.
	// distribute ns to p and done secondaries
	err := dst.Put(o)
	if err != nil {
		return err
	}
	log.Printf("%s(%s) -> %s\n", src, id, dst)
	for _, d := range secondaries {
		err = d.Put(o)
		if err != nil {
			log.Printf("Failed to sync %v to secondary %#v: %v\n", id, d, err)
		}
		log.Printf("%s(%s) -> %s\n", src, id, d)
	}
	// TODO: should we collect errors from secondaries and return them here? We don't want to
	// return on errors sending to secondaries, but those *ARE* errors that will result in an
	// incomplete sync.
	return nil
}

// Sync causes the Controller to synchronize all of the Storage objects it is managing. This
// ensures that they have the same set of Objects with the same sets of tags.
//
// Sync may take a long time to complete as it scans the entire object set on every Storage.
//
// A Sync may be safely interrupted by a call to Close().
func (c *Controller) Sync() error {
	c.plock.Lock()
	defer c.plock.Unlock()
	c.slock.Lock()
	defer c.slock.Unlock()

	var done []Storage
	for _, s := range c.secondaries {
		pi, err := c.primary.Index().Iter()
		if err != nil {
			return err
		}
		si, err := s.Index().Iter()
		if err != nil {
			log.Printf("Failed to iterate index for %#v: %v, Skipping.", s, err)
			continue
		}
		pid, perr := pi.Next()
		sid, serr := si.Next()
		for {
			if !c.isOpen() {
				log.Printf("Controller shut down. Stopping sync.")
				return nil
			}
			if perr != nil && perr != itDone {
				return err
			}
			if serr != nil && serr != itDone {
				log.Printf("Failed to iterate index for %#v: %v, Skipping.", s, err)
				break
			}

			if perr == itDone && serr == itDone {
				done = append(done, s)
				break
			} else if perr == itDone {
				log.Printf("Syncing %s", sid)
				err = copyObject(c.primary, s, done, sid)
				if err != nil {
					return err
				}
				sid, serr = si.Next()
				continue
			} else if serr == itDone {
				log.Printf("Syncing %s", pid)
				err = copyObject(s, c.primary, nil, pid)
				if err != nil {
					log.Printf("Failed to sync %v to secondary %#v: %v\n", pid, s, err)
				}
				pid, perr = pi.Next()
				continue
			}

			i := strings.Compare(pid, sid)
			if i == 0 {
				// The IDs are present in both.
				// TODO(kjn): sync the hashes/tags
				log.Printf("Syncing %s", pid)
				pid, perr = pi.Next()
				sid, serr = si.Next()
				continue
			} else if i > 0 {
				log.Printf("Syncing %s", sid)
				err = copyObject(c.primary, s, done, sid)
				if err != nil {
					return err
				}
				sid, serr = si.Next()
				continue
			} else if i < 0 {
				log.Printf("Syncing %s", pid)
				err = copyObject(s, c.primary, nil, pid)
				if err != nil {
					log.Printf("Failde to sync %v to secondary %#v: %v\n", pid, s, err)
				}
				pid, perr = pi.Next()
				continue
			}
		}
	}
	log.Printf("Successfully synced storage.\n")
	c.primary.SetCheckpoint(0)
	for _, s := range c.secondaries {
		s.SetCheckpoint(0)
	}
	c.journal.Reset()
	return nil
}

func (c *Controller) Put(o Object) error {
	c.plock.Lock()
	defer c.plock.Unlock()
	err := c.primary.Put(o)
	if err != nil {
		return err
	}

	i, err := c.journal.Add(event{T: ev_put, ID: o.ID()})
	if err != nil {
		// This should only happen if the journal has been closed. That should never happen, but if it
		// does, secondaries will get out of sync. That can only be caught by a manual sync.
		return err
	}

	c.primary.SetCheckpoint(i)
	return nil
}

func (c *Controller) pcheckpt() uint64 {
	c.plock.RLock()
	defer c.plock.RUnlock()
	return c.primary.Checkpoint()
}

func (c *Controller) syncJournals() bool {
	log.Printf("Syncing journals.\n")
	defer log.Printf("Finished syncing journals.\n")
	pi := c.pcheckpt()
	c.slock.Lock()
	defer c.slock.Unlock()
	deadline := time.Now().Add(5 * time.Second)
sloop:
	for _, s := range c.secondaries {
		si := s.Checkpoint()
		if si < pi {
			for i := si + 1; i <= pi; i++ {
				if !c.isOpen() {
					return false
				}
				if time.Now().After(deadline) {
					return true
				}
				fmt.Printf("%s syncing event %d\n", s, i)
				ev, err := c.journal.Get(i)
				if err != nil {
					log.Printf("Failed to sync event %#v to secondary %s: %v\n", ev, s, err)
					continue sloop
				}
				if ev.T == ev_put {
					o, ok := c.primary.Get(ev.ID)
					if !ok {
						log.Printf("Failed to sync event %#v to secondary %s: object %s not present.\n", ev, s, ev.ID)
						continue sloop
					}
					err := s.Put(o)
					if err != nil {
						log.Printf("Failed to sync event %#v to secondary %s: %v\n", ev, s, err)
						continue sloop
					}
					s.SetCheckpoint(i)
					continue
				} else {
					log.Printf("CANNOT SYNC EVENT OF TYPE %#v\n", ev.T)
					continue sloop
				}
				// To make sure we never accidentally skip an event, we only write the checkpoint exactly where we
				// have executed the event on the secondary, at which point we also advance to the next loop
				// iteration. If we ever make it here, that means our logic has failed and we didn't manage to sync
				// the event, so we'll skip this secondary and move on to the next one.
				log.Printf("Somehow failed to sync event %#v. This is a server logic error. Skipping to next secondary.")
				continue sloop
			}
		}
	}
	return false
}

func (c *Controller) worker() {
	log.Printf("Controller worker starting.")
	defer log.Printf("Controller worker shutting down.")
	c.wg.Add(1)
	defer c.wg.Done()
	t := time.NewTicker(c.period)
	defer t.Stop()
	for {
		select {
		case <-c.shutdown:
			return
		default:
		}
		select {
		case <-c.shutdown:
			return
		case <-t.C:
			if c.syncJournals() {
				t.Reset(1 * time.Second)
			} else {
				t.Reset(c.period)
			}
		}
	}
}

func (c *Controller) isOpen() bool {
	select {
	case <-c.shutdown:
		return false
	default:
		return true
	}
}

func (c *Controller) tryShutdown() bool {
	var ret bool
	c.sdo.Do(func() {
		ret = true
		close(c.shutdown)
	})
	return ret
	// 	c.plock.Lock()
	// 	defer c.plock.Unlock()
	// 	c.slock.Lock()
	// 	defer c.slock.Unlock()
	// 	if !c.isOpen() {
	// 		return false
	// 	}
	// 	close(c.shutdown)
	// 	return true
}

func (c *Controller) Close() error {
	sd := c.tryShutdown()
	if !sd {
		return nil
	}
	log.Printf("Shutting down Controller.")
	c.journal.Close()
	c.wg.Wait()
	return nil
}
