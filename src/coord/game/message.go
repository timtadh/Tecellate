package game

import pseudo_rand "rand"
import crypto_rand "crypto/rand"
import "sort"
import geo "coord/geometry"

const MessageLength = 256
const perfectHear = 5.0

type Message interface {
    Source() *geo.Point
    Message() []byte
    Frequency() uint8
}

type sortableMessages struct {
    msgs []Message
    targ *geo.Point
}

type Messages map[uint8][]Message

// initializer for random number generator -------------------------------------
func init() {
    // This function was a originally part of the structure/block/byteslice pkg
    // in SourceQL
    Int64 := func (b []byte) int64 {
        i := uint64(0)
        for j := 0; j < len(b) && j < 8; j++ {
            i |= 0x00000000000000ff & uint64(b[j])
            if j+1 < len(b) {
                i <<= 8
            }
        }
        return int64(i)
    }
    pseudo_rand.Seed(Int64(randbytes(8)))
}

// convience functions ---------------------------------------------------------
func randbyte() byte {
    return randbytes(1)[0]
}
func randbytes(k int) []byte {
    bytes := make([]byte, k)
    if n, err := crypto_rand.Read(bytes); n == k && err == nil {
        return bytes
    }
    panic("Can't get random byte.")
}
func corrupt(msg []byte, dist float64) (corrupted []byte) {
    corrupted = make([]byte, MessageLength)
    for i := 0; i < MessageLength; i++ {
        var cur byte
        if i < len(msg) { cur = msg[i] } else { cur = randbyte() }
        if dist > perfectHear && pseudo_rand.Float64() > 10.0/dist {
            corrupted[i] = cur ^ randbyte()
        } else {
            corrupted[i] = cur
        }
    }
    return
}

// Messages Methods -----------------------------------------------------------
func (self Messages) Add(msg Message) {
    f := msg.Frequency()
    if _, has := self[f]; !has {
        self[f] = make([]Message, 0, 10)
    }
    self[f] = append(self[f], msg)
}

func (self Messages) Hear(loc *geo.Point, freq uint8) (msg []byte) {
    msg = make([]byte, MessageLength)
    if messages, has := self[freq]; has {
        msgs := newSortableMessages(len(messages), loc)
        for _, msg := range messages {
            msgs.add(msg)
        }
        msgs.sort()
        for i, M := range msgs.msgs {
            dist := M.Source().Distance(loc)
            m := corrupt(M.Message(), dist)
            if i == 0 {
                msg = m
            } else {
                for j, byt := range m {
                    if pseudo_rand.Float64() > 10.0/dist {
                        msg[j] = msg[j] ^ byt
                    }
                }
            }
        }
        return
    }
    return randbytes(MessageLength)
}

// messageSlice Methods --------------------------------------------------------
func newSortableMessages(size int, loc *geo.Point) *sortableMessages {
    return &sortableMessages{
            msgs:make([]Message, 0, size),
            targ:loc,
    }
}
func (self *sortableMessages) add(msg Message) {
    self.msgs = append(self.msgs, msg)
}
func (self *sortableMessages) sort() *sortableMessages {
    sort.Sort(self)
    return self
}

// sort interface
func (self *sortableMessages) Len() int { return len(self.msgs) }
func (self *sortableMessages) Swap(i, j int) {
    self.msgs[i], self.msgs[j] = self.msgs[j], self.msgs[i]
}
func (self *sortableMessages) Less(i, j int) bool {
    a := self.msgs[i].Source().DistanceSquare(self.targ)
    b := self.msgs[j].Source().DistanceSquare(self.targ)
    return a < b
}
