package lib

import "fmt"
import pseudo_rand "rand"
import "agent"
import "logflow"
import . "byteslice"

import . "agents/wifi/lib/datagram"
import . "agents/wifi/lib/packet"

const SEND_HOLDTIME = 15
const ACK_WAIT = 20

type SendMachine struct {
    freq uint8
    agent agent.Agent
    logger logflow.Logger
    last_checksum ByteSlice
    last ByteSlice
    heard_last bool
    state uint32
    backoff float64
    wait uint32
    ack_wait uint32
    ack_backoff float64
    next_state uint32
    routes RoutingTable
    recieve ByteSlice
    sendq *DataGramQueue
}


func NewSendMachine(freq uint8, agent agent.Agent) *SendMachine {
    self := &SendMachine {
        freq:freq,
        logger:logflow.NewSource(fmt.Sprintf("agent/wifi/send/%d", agent.Id())),
        agent:agent,
        backoff:BACKOFF,
        wait:ROUTE_HOLDTIME,
        state:2,
        next_state:0,
        sendq:NewDataGramQueue(),
    }
    return self
}

func (self *SendMachine) Run(routes RoutingTable, comm agent.Comm) *DataGram {
    self.routes = routes
    m := self.PerformListens(comm)
    self.PerformSends(comm)
    self.sendq.Clean()
    return m
}

func (self *SendMachine) Send(msg ByteSlice, dest uint32) {
    gram := NewDataGram(msg, uint32(self.agent.Id()), dest)
    self.sendq.Queue(gram)
//     self.log("info", self.agent.Time(), "Put message on sendq", string([]byte(msg)), gram)
}

func (self *SendMachine) log(level logflow.LogLevel, v ...interface{}) {
    self.logger.Logln(level, v...)
}

func (self *SendMachine) qlog(v ...interface{}) {
    args := make([]interface{}, len(v)+1)
    args[0] = self.agent.Time()
    copy(args[1:], v)
    self.log("info", args...)
}

func (self *SendMachine) confirm_last(comm agent.Comm) (confirm bool) {
    bytes := comm.Listen(self.freq)
    confirm = self.last.Eq(bytes)
    self.log("info", self.agent.Time(), "confirm_last", confirm)
    return
}

func (self *SendMachine) confirm_acked(comm agent.Comm) (confirm bool) {

    cur := comm.Listen(self.freq)
    if self.last.Eq(cur) {
        self.qlog("heard my own packet")
        self.heard_last = true
        return false
    }

    if self.heard_last == false {
        self.wait -= 1
        if self.wait == 0 {
            self.state = 0
            self.backoff = self.backoff*(pseudo_rand.Float64()*2 + 1)
            self.wait = uint32(self.backoff)
            self.qlog("didn't hear last waiting...", self.wait)
        }
        return false
    }

    self.ack_wait -= 1
    if self.ack_wait == 0 {
        self.ack_backoff = self.ack_backoff*(pseudo_rand.Float64()*2 + 1)
        self.ack_wait = uint32(self.ack_backoff)
        self.state = 0
        return false
    }

    pkt := MakePacket(cur)
    if !pkt.ValidateChecksum() { return false }
    ok, cmd, _ := pkt.Cmd()
    if !ok {  return false }
    self.log("info", self.agent.Time(), "got possible ack pkt", pkt)
    switch cmd {
        case Commands["ACK"]:
            self.log("info", self.agent.Time(), "it is an ack")
            myaddr := self.agent.Id()
            to := pkt.ToField()
            body := pkt.GetBody(PacketBodySize)
            self.log("info", self.agent.Time(), "it is to me?", myaddr, to)
            if to == myaddr {
                msg := MakeDataGram(body)
                self.log("info", self.agent.Time(), "the datagram", msg)
                if msg.ValidateChecksum() {
                    self.log("info", self.agent.Time(), "it validates")
                    self.log("info", self.agent.Time(), "it dest", msg.DestAddr, "myaddr", myaddr)
                    if msg.DestAddr == myaddr {
                        self.log("info", self.agent.Time(), "it dest == myaddr")
                        bytes := msg.Body()[0:4]
                        self.log("info", self.agent.Time(), "bytes", bytes, "last", self.last_checksum)
                        if self.last_checksum.Eq(bytes) {
                            return true
                        }
                    }
                }
            }
    }
    return false
}

func (self *SendMachine) send_message(comm agent.Comm) (sent, isack bool) {

    find := func() (*DataGram, bool) {
        for i := 0; i < self.sendq.Len(); i++ {
            msg, ok := self.sendq.Dequeue()
            if !ok { break; }
            if _, has := self.routes[msg.DestAddr]; !has {
                self.sendq.Queue(msg)
            } else {
                return msg, true
            }
        }
        return nil, false
    }
    msg, found := find()
    if !found { return false, false }

    self.sendq.QueueFront(msg)

    next := self.routes[msg.DestAddr].NextAddr
    var pkt *Packet
    if msg.IsAck() {
        pkt = NewPacket(Commands["ACK"], self.agent.Id(), next)
    } else {
        pkt = NewPacket(Commands["MESSAGE"], self.agent.Id(), next)
    }
    pkt.SetBody(msg.Bytes())
    bytes := pkt.Bytes()
    comm.Broadcast(self.freq, bytes)
    if !msg.IsAck() {
        self.last_checksum = msg.ComputeChecksum()
    }
    self.last = bytes
    self.log("info", self.agent.Time(), "sent", pkt, msg)
    return true, msg.IsAck()
}

func (self *SendMachine) PerformSends(comm agent.Comm) {
    if self.agent.Time() >= 750 && self.agent.Time() <= 1000 {
        self.log("info", self.agent.Time(), "current state", self.state)
    }
    switch self.state {
        case 0:
            self.heard_last = false
            sent, isack := self.send_message(comm)
            if sent && !isack {
                self.state = 1
            } else if sent {
                self.state = 4
            } else {
                self.state = 3
            }
        case 1:
            if self.confirm_acked(comm) {
                if !self.sendq.Empty() {
                    m, _ := self.sendq.Dequeue()
                    self.log("info", self.agent.Time(), "dequeued msg", m)
                }
                self.state = 2
                self.next_state = 3
                self.backoff = BACKOFF
                self.wait = SEND_HOLDTIME
            }
        case 2:
            self.wait -= 1
            if self.wait == 0 {
                self.state = self.next_state
            }
        case 3:
            if !self.sendq.Empty() {
                self.state = 0
                self.backoff = BACKOFF
                self.ack_backoff = ACK_WAIT
                self.wait = uint32(self.backoff)
                self.ack_wait = uint32(self.ack_backoff)
            }
        case 4:
            self.next_state = 3
            if self.confirm_last(comm) {
                if !self.sendq.Empty() {
                    self.sendq.Dequeue()
                }
                self.backoff = BACKOFF
                self.state = 2
                self.wait = 1
            } else {
                self.state = 2
                self.backoff = self.backoff*(pseudo_rand.Float64()*2 + 1)
                self.wait = uint32(self.backoff)
            }
        default:
//             self.log("debug", self.agent.Time(), "nop")
    }
}

func (self *SendMachine) PerformListens(comm agent.Comm) *DataGram {
    switch self.state {
        case 1:
            return nil
    }
    pkt := MakePacket(comm.Listen(self.freq))
    if !pkt.ValidateChecksum() { return nil }
    ok, cmd, _ := pkt.Cmd()
    if !ok { return nil }
    switch cmd {
        case Commands["MESSAGE"]:
            myaddr := uint32(self.agent.Id())
            to := pkt.ToField()
            from := pkt.FromField()
            body := pkt.GetBody(PacketBodySize)
            self.qlog("heard", from, "sent to", to, "pkt", pkt, MakeDataGram(body))
            if to == myaddr {
                msg := MakeDataGram(body)
                if msg.ValidateChecksum() {
                    ack := NewAckGram(msg.ComputeChecksum(), myaddr, from)
                    self.sendq.QueueFront(ack)
//                     ack_pkt := NewPacket(Commands["ACK"], myaddr, from)
//                     ack_pkt.SetBody(ack.Bytes())
//                     bytes := ack_pkt.Bytes()
//                     comm.Broadcast(self.freq, bytes)
                    if msg.DestAddr == myaddr {
                        return msg
                    }
                    self.sendq.Queue(msg)
                }
            }
    }
    return nil
}
