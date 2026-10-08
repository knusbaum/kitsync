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
	ev_delete
	ev_add_tag
	ev_del_tag
	ev_sync_tag
)

type event struct {
	T    evtype
	ID   string
	Tagk string
	Tagv string
}

type Controller struct {
	primary     ControllableStorage
	secondaries []ControllableStorage
	needSync    []ControllableStorage
	journal     *journal.Journal[event]

	period   time.Duration
	sdo      sync.Once
	shutdown chan struct{}

	plock sync.RWMutex // lock on the primary storage
	slock sync.RWMutex // lock on the secondary storage
	wg    sync.WaitGroup
}

var _ Storage = &Controller{}

// NewController creates a new controller for the primary Storage. Local data such as the journal
// is stored in the directory at dir.
func NewController(dir string, primary ControllableStorage) (*Controller, error) {
	c := &Controller{
		primary:  primary,
		period:   10 * time.Second,
		shutdown: make(chan struct{}),
	}
	jpath := path.Join(dir, "journal")
	if _, err := os.Stat(jpath); os.IsNotExist(err) {
		fmt.Printf("Creating journal with %d entries.\n", 2_000_000)
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

func (c *Controller) AddSecondary(s ControllableStorage) {
	c.slock.Lock()
	defer c.slock.Unlock()
	c.secondaries = append(c.secondaries, s)
	log.Printf("Added secondary %s at checkpoint %d\n", s, s.Checkpoint())
}

func copyObject(dst, src ControllableStorage, secondaries []ControllableStorage, id string) error {
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
	defer log.Printf("Copy Done.\n")
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

	var done []ControllableStorage
	for _, s := range c.secondaries {
		pi, err := c.primary.Iter()
		if err != nil {
			return err
		}
		si, err := s.Iter()
		if err != nil {
			log.Printf("Failed to iterate index for %#v: %v, Skipping.", s, err)
			continue
		}
		defer si.Close()

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
				log.Printf("Syncing %s to primary", sid)
				err = copyObject(c.primary, s, done, sid)
				if err != nil {
					return err
				}
				sid, serr = si.Next()
				continue
			} else if serr == itDone {
				log.Printf("Syncing %s to secondary", pid)
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
				//log.Printf("Syncing %s", pid)
				pid, perr = pi.Next()
				sid, serr = si.Next()
				continue
			} else if i > 0 {
				log.Printf("Syncing %s to primary", sid)
				err = copyObject(c.primary, s, done, sid)
				if err != nil {
					return err
				}
				sid, serr = si.Next()
				continue
			} else if i < 0 {
				log.Printf("Syncing %s to secondary", pid)
				err = copyObject(s, c.primary, nil, pid)
				if err != nil {
					log.Printf("Failed to sync %v to secondary %#v: %v\n", pid, s, err)
				}
				pid, perr = pi.Next()
				continue
			}
		}
	}

	log.Printf("Successfully synced storage.\n")
	// 	//i, err := c.journal.Add(event{T: ev_sync_tag})
	// 	if err != nil {
	// 		// This should only happen if the journal has been closed. That should never happen, but if it
	// 		// does, secondaries will get out of sync. That can only be caught by a manual sync.
	// 		return err
	// 	}
	i := c.primary.Checkpoint()
	for _, s := range c.secondaries {
		s.SetCheckpoint(i)
	}
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
	pi := c.pcheckpt()
	c.slock.Lock()
	defer c.slock.Unlock()
	deadline := time.Now().Add(5 * time.Second)
sloop:
	for secondaryi, s := range c.secondaries {
		si := s.Checkpoint()
		if si < pi {
			log.Printf("Syncing %s with primary.", s)
			for i := si + 1; i <= pi; i++ {
				if !c.isOpen() {
					log.Printf("Journal sync interrupted.")
					return false
				}
				if time.Now().After(deadline) {
					return true
				}
				//fmt.Printf("%s syncing event %d\n", s, i)
				ev, err := c.journal.Get(i)
				if err != nil {
					if err == journal.ErrNotPresent {
						// This secondary is too old, and is on a journal entry that has been overwritten.
						// We need to do a full sync.
						c.needSync = append(c.needSync, s)
						c.secondaries = append(c.secondaries[:secondaryi], c.secondaries[secondaryi+1:]...)
						return true
					}
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
				} else if ev.T == ev_delete {
					err := s.Delete(ev.ID)
					if err != nil {
						log.Printf("Failed to sync event %#v to secondary %s: %v\n", ev, s, err)
						continue sloop
					}
					s.SetCheckpoint(i)
					continue
				} else if ev.T == ev_add_tag {
					o, ok := s.Get(ev.ID)
					if !ok {
						log.Printf("Failed to sync event %#v to secondary %s: object %s not present.\n", ev, s, ev.ID)
						continue sloop
					}
					err := o.AddTag(ev.Tagk, ev.Tagv)
					if err != nil {
						log.Printf("Failed to sync event %#v to secondary %s: %v\n", ev, s, err)
						continue sloop
					}
					s.SetCheckpoint(i)
					continue
				} else if ev.T == ev_del_tag {
					o, ok := s.Get(ev.ID)
					if !ok {
						log.Printf("Failed to sync event %#v to secondary %s: object %s not present.\n", ev, s, ev.ID)
						continue sloop
					}
					err := o.DelTag(ev.Tagk)
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
			}
		}
	}
	return false
}

// see: syncToSecondaries
func (c *Controller) syncLoop(toSync []ControllableStorage) (done []ControllableStorage, fail []ControllableStorage) {
	fmt.Printf("Sync loop %#v\n", toSync)
	defer fmt.Printf("Sync Loop done.\n")

	checkpoint := c.primary.Checkpoint()
	pi, err := c.primary.Iter()
	if err != nil {
		return nil, toSync
	}

	pid, perr := pi.Next()
	for {
		if !c.isOpen() {
			log.Printf("Controller shut down. Stopping sync.")
			return nil, append(toSync, fail...)
		}
		if perr != nil && perr != itDone {
			log.Printf("Failed to iterate primary storage: %s\n", err)
			return nil, append(toSync, fail...)
		}
		if perr == itDone {
			fmt.Printf("Iterator DONE.\n")
			break
		}
		if len(toSync) == 0 {
			fmt.Printf("No More Tosync.\n")
			break
		}

		k := 0
		for si, s := range toSync {
			log.Printf("Syncing %s to secondary", pid)
			err = copyObject(s, c.primary, nil, pid)
			if err != nil {
				log.Printf("Failed to sync %v to secondary %#v: %v\n", pid, s, err)
				fail = append(fail, s)
			}
			toSync[k] = toSync[si]
			k++
		}
		toSync = toSync[:k]
		pid, perr = pi.Next()
	}
	for _, s := range toSync {
		s.SetCheckpoint(checkpoint)
	}
	return toSync, fail
}

// syncToSecondaries performs a one-way sync from the primary storage to out-of-sync secondaries.
func (c *Controller) syncToSecondaries() {
	fmt.Printf("sync to secondaries\n")
	defer fmt.Printf("Done sync to secondaries.\n")
	c.slock.Lock()
	syncs := make([]ControllableStorage, len(c.needSync))
	copy(syncs, c.needSync)
	c.needSync = nil
	c.slock.Unlock()
	done, fail := c.syncLoop(syncs)

	c.slock.Lock()
	c.secondaries = append(c.secondaries, done...)
	c.needSync = append(c.needSync, fail...)
	c.slock.Unlock()
}

func (c *Controller) worker() {
	log.Printf("Controller worker starting.")
	defer log.Printf("Controller worker shutting down.")
	c.wg.Add(1)
	defer c.wg.Done()
	t := time.NewTicker(c.period)
	defer t.Stop()
	synct := time.NewTicker(c.period * 2)
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
			// fmt.Printf("Tick\n")
			if c.syncJournals() {
				t.Reset(1 * time.Second)
			} else {
				t.Reset(c.period)
			}
			if len(c.needSync) > 0 {
				fmt.Printf("%d secondaries need sync: %v\n", len(c.needSync), c.needSync)
			}
		case <-synct.C:
			c.slock.RLock()
			// fmt.Printf("Sync Tick\n")
			if len(c.needSync) > 0 {
				go c.syncToSecondaries()
			}
			c.slock.RUnlock()
			c.plock.Lock()
			c.slock.Lock()
			c.primary.Cleanup()
			//fmt.Printf("Cleaning up.\n")
			for _, s := range c.secondaries {
				s.Cleanup()
			}
			c.slock.Unlock()
			c.plock.Unlock()
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

func (c *Controller) Iter() (Iterator, error) {
	return c.primary.Iter()
}

func (c *Controller) Get(id string) (Object, bool) {
	o, ok := c.primary.Get(id)
	if !ok {
		return nil, false
	}
	return &cob{o, c}, true
}

func (c *Controller) Delete(id string) error {
	err := c.primary.Delete(id)
	if err != nil {
		return err
	}
	i, err := c.journal.Add(event{T: ev_delete, ID: id})
	if err != nil {
		// This should only happen if the journal has been closed. That should never happen, but if it
		// does, secondaries will get out of sync. That can only be caught by a manual sync.
		return err
	}
	c.primary.SetCheckpoint(i)
	return nil
}

func (c *Controller) Present(id string) bool {
	return c.Present(id)
}

func (c *Controller) String() string {
	var str strings.Builder
	fmt.Fprintf(&str, "Controller(")
	fmt.Fprintf(&str, "%s", c.primary)
	for _, s := range c.secondaries {
		fmt.Fprintf(&str, ", %s", s)
	}
	fmt.Fprintf(&str, ")")
	return str.String()
}

type cob struct {
	Object
	c *Controller
}

func (c *cob) AddTag(k, v string) error {
	err := c.Object.AddTag(k, v)
	if err != nil {
		return err
	}
	i, err := c.c.journal.Add(event{T: ev_add_tag, ID: c.ID(), Tagk: k, Tagv: v})
	if err != nil {
		// This should only happen if the journal has been closed. That should never happen, but if it
		// does, secondaries will get out of sync. That can only be caught by a manual sync.
		return err
	}
	c.c.primary.SetCheckpoint(i)
	return nil
}

func (c *cob) DelTag(k string) error {
	err := c.Object.DelTag(k)
	if err != nil {
		return err
	}
	i, err := c.c.journal.Add(event{T: ev_del_tag, ID: c.ID(), Tagk: k})
	if err != nil {
		// This should only happen if the journal has been closed. That should never happen, but if it
		// does, secondaries will get out of sync. That can only be caught by a manual sync.
		return err
	}
	c.c.primary.SetCheckpoint(i)
	return nil
}
