package proxy

import (
    "fmt"
    "time"
    "logflow"
)
import (
    "agent"
    "agent/link"
    cagent "coord/agent"
    "coord/game"
    geo "coord/geometry"
)

import . "coord/agent"

type AgentProxy struct {
    state *AgentState
    snd link.SendLink
    rcv link.RecvLink
    log logflow.Logger
    game *game.GameState
}

func NewAgentProxy(send link.SendLink, recv link.RecvLink) *AgentProxy {
    self := new(AgentProxy)
//     self.state = NewAgentState(0, geo.NewPoint(0, 0), 0)
    self.snd = send
    self.rcv = recv
    self.log = logflow.NewSource("agentproxy/?")
    return self
}

func RunAgentLocal(a agent.Agent, x, y, energy int) *AgentProxy {
    p2a := make(chan link.Message, 10)
    a2p := make(chan link.Message, 10)

    proxy := NewAgentProxy(p2a, a2p)
    proxy.SetState(cagent.NewAgentState(a.Id(), 0, *geo.NewPoint(x, y), Energy(energy)))
    go func() {
        agent.Run(a, a2p, p2a)
    }()
    return proxy
}

func (self *AgentProxy) SetState(state *AgentState) {
    self.state = state
    self.log = logflow.NewSource(fmt.Sprintf("agentproxy/%v", self.state.Id))
}

func (self *AgentProxy) State() *AgentState {
    return self.state
}

func (self *AgentProxy) Apply(trans Transform) {
    self.state.Transform(trans)
}

func (self *AgentProxy) SetGameState(g *game.GameState) {
    self.game = g
}

func (self *AgentProxy) Turn() bool {
    type handler (func(*link.Message) bool)

    check_args := func(count int, args link.Arguments) bool {
        if len(args) == count {
            return true
        }
        self.log.Println("Error : Wrong number of arguments recieved")
        return false
    }

    argnum := func(count int, f handler) handler {
        return func(msg *link.Message) bool {
            if check_args(count, msg.Args) {
                return f(msg)
            }
            return false
        }
    }

    var handlers = map[link.Command]handler {
        link.Commands["Complete"]:
            argnum(0, func(msg *link.Message) bool {
                self.ack_cmd(msg.Cmd)
                return true
            }),
        link.Commands["Move"]:
            argnum(1, func(msg *link.Message) bool {
                mv := geo.MakePoint(msg.Args[0])
                if self.state.Mv(mv) {
                    self.ack_cmd(msg.Cmd)
                } else {
                    self.nak_cmd(msg.Cmd)
                }
                return false
            }),
        link.Commands["Look"]:
            argnum(0, func(msg *link.Message) bool {
                self.send(link.NewMessage(link.Commands["Ack"], msg.Cmd, nil))
                return false
            }),
        link.Commands["Listen"]:
            argnum(1, func(msg *link.Message) bool {
                freq := agent.MakeListen(msg.Args[0]).Listen()
                heard := self.game.Listen(self.state.Position, freq)
                self.send(link.NewMessage(link.Commands["Ack"], msg.Cmd, heard))
                return false
            }),
        link.Commands["Broadcast"]:
            argnum(1, func(msg *link.Message) bool {
                freq, pkt := agent.MakeBroadcast(msg.Args[0]).Message()
                if self.state.Broadcast(freq, pkt) {
                    self.ack_cmd(msg.Cmd)
                } else {
                    self.nak_cmd(msg.Cmd)
                }
                return false
            }),
        link.Commands["Collect"]:
            argnum(0, func(msg *link.Message) bool {
                if self.state.Collect() {
                    self.ack_cmd(msg.Cmd)
                } else {
                    self.nak_cmd(msg.Cmd)
                }
                return false
            }),
        link.Commands["Energy"]:
            argnum(0, func(msg *link.Message) bool {
                self.send(link.NewMessage(link.Commands["Ack"], msg.Cmd, self.state.Energy))
                return false
            }),
        link.Commands["PrevResult"]:
            argnum(0, func(msg *link.Message) bool {
//                 self.send(link.NewMessage(link.Commands["Ack"], msg.Cmd, self.state.PrevResult()))
                return false
            }),
    }

    handle := func(msg *link.Message) bool {
        if f, ok := handlers[msg.Cmd]; ok {
            return f(msg)
        }
        panic(fmt.Sprintf("Command %s not found.", msg.Cmd))
    }

    complete := make(chan bool)
    if self.game == nil {panic("(agentproxy) self.game == nil")}
    self.state.NewMove()
    if !self.state.Alive { return false }
//     self.log.Println("Starting Turn", self.state.Turn)
//     self.log.Println(self.state)
    if !self.start_turn() {
        return false
    }
    go func(done chan<- bool) {
        for {
            if ok, msg := self.recv(); ok {
                if handle(msg) {
                    done <- true
                    break
                }
            } else {
                    done <- false
                    break
            }
        }
        return
    }(complete)
    c := <-complete
//     self.log.Println("Ending Turn", self.state.Turn)
    return c
}

func (self *AgentProxy) MigrateTo(address string) {
    self.acked_send(link.NewMessage(link.Commands["Migrate"], agent.NewMigrate([]byte(address))))
}

func (self *AgentProxy) start_turn() bool {
    return self.acked_send(link.NewMessage(link.Commands["Start"], self.state.Turn))
}

func (self *AgentProxy) Stop() {
    self.send(link.NewMessage(link.Commands["Exit"]))
}

func (self *AgentProxy) ack_cmd(cmd link.Command) {
    self.send(link.NewMessage(link.Commands["Ack"], cmd))
}

func (self *AgentProxy) nak_cmd(cmd link.Command) {
    self.send(link.NewMessage(link.Commands["Nak"], cmd))
}

func (self *AgentProxy) recv() (bool, *link.Message) {
    timeout := time.NewTicker(link.Timeout)
    select {
    case msg := <-self.rcv:
//         self.log.Logf("proto", "recv : %v", msg)
        return true, &msg
    case <-timeout.C:
        timeout.Stop()
        self.log.Println("Client unresponsive.")
    }
    return false, nil
}

func (self *AgentProxy) send(msg *link.Message) bool {
    timeout := time.NewTicker(link.Timeout)
    select {
    case m := <-self.rcv:
        self.log.Println("recv unresolved message", m)
    case self.snd <- *msg:
//         self.log.Logf("proto", "sent : %v", msg)
        return true
    case <-timeout.C:
        timeout.Stop()
        self.log.Println("Client unresponsive.")
    }
    return false
}

func (self *AgentProxy) acked_send(msg *link.Message) bool {
    self.send(msg)
    return self.await_cmd_ack(msg.Cmd)
}

func (self *AgentProxy) await_cmd_ack(cmd link.Command) bool {
    if ok, msg := self.recv(); ok {
        if msg.Cmd == link.Commands["Ack"] && len(msg.Args) == 1 {
            if acked := link.MakeCommand(msg.Args[0]); acked == cmd {
                return true
            }
        }
    }
    return false
}
