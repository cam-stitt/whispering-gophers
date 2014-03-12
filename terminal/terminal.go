package terminal

import (
    "bufio"
    "encoding/json"
    "log"
    "fmt"
    "net"
    "os"
    "sync"
    "io"
    "strings"
    "time"

    "github.com/pdxgo/whispering-gophers/util"
)

var (
    PeerAddr string
    BindPort int
    Self     string
    SelfNick string
    discPort int = 5555
    Peers = &Peer{m: make(map[string]chan<- Message)}
)

// Defines a single message sent from one peer to another
type Message struct {
    // Random ID for each message used to prevent re-broadcasting messages
    ID string
    // IP:Port combination the peer who sent a message is listening on
    Addr string
    // Actual message to display
    Body string
    // Nickname
    Nick string `json:"omitempty"`

    // In Unix Timestampe format
    Timestamp int64
}

type Peer struct {
    m  map[string]chan<- Message
    mu sync.RWMutex
}

// Add creates and returns a new channel for the given peer address.
// If an address already exists in the registry, it returns nil.
func (p *Peer) Add(addr string) <-chan Message {
    p.mu.Lock()
    defer p.mu.Unlock()
    if _, ok := p.m[addr]; ok {
        return nil
    }
    ch := make(chan Message)
    p.m[addr] = ch
    return ch
}

// Remove deletes the specified peer from the registry.
func (p *Peer) Remove(addr string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    delete(p.m, addr)
}

// List returns a slice of all active peer channels.
func (p *Peer) List() []chan<- Message {
    p.mu.RLock()
    defer p.mu.RUnlock()
    l := make([]chan<- Message, 0, len(p.m))
    for _, ch := range p.m {
        l = append(l, ch)
    }
    return l
}

func (p *Peer) Get() []string {
    p.mu.RLock()
    defer p.mu.RUnlock()
    out := make([]string, len(p.m))
    for peer, _ := range p.m {
        out = append(out, peer)
    }
    return out
}

func broadcast(m Message) {
    for _, ch := range Peers.List() {
        ch <- m
    // Never drop the message!!!!!!
  }
}

func Serve(c net.Conn) {
    defer c.Close()
    d := json.NewDecoder(c)
    for {
        var m Message
        err := d.Decode(&m)
        if err != nil {
            if err == io.EOF {
                log.Printf("%s disconnected. %d Peers remaining.", c.RemoteAddr(), len(Peers.m))
            } else {
                log.Printf("%s disconnected with %v. %d Peers remaining.", c.RemoteAddr(), err, len(Peers.m))
            }
            return
        }
        if Seen(m.ID) {
            continue
        }
        nick := m.Nick
        if nick == "" {
            nick = m.Addr
        }
        if m.Body[0] == '/' {
            handleCommand(&m)
        } else {
            fmt.Printf("%s: %s\n", nick, m.Body)
        }
        broadcast(m)
        go Dial(m.Addr)
    }
}

func createMessage(m string) Message {
    return Message{
        ID:        util.RandomID(),
        Addr:      Self,
        Body:      m,
        Nick:      SelfNick,
        Timestamp: time.Now().Unix(),
    }
}

func handleCommand(m *Message) {
    if strings.HasPrefix(m.Body, "/me ") {
        fmt.Printf("%s %s\n", m.Nick, m.Body[4:])
    }
}

func doCommand(command string) {
    if strings.HasPrefix(command, "/connect ") {
        addr := strings.TrimLeft(command, "/connect ")
        go Dial(addr)
    }
}

func ReadInput() {
    s := bufio.NewScanner(os.Stdin)
    for s.Scan() {
        body := s.Text()
        if body != "" {
            if body[0] == '/' {
                doCommand(body)
            } else {
                m := createMessage(body)
                Seen(m.ID)
                broadcast(m)
            }
        }
    }
    if err := s.Err(); err != nil {
        log.Fatal(err)
    }
}

func Dial(addr string) {
    if addr == Self {
        return // Don't try to dial self.
    }

    ch := Peers.Add(addr)
    if ch == nil {
        return // Peer already connected.
    }
    defer Peers.Remove(addr)

    c, err := net.Dial("tcp", addr)
    if err != nil {
        log.Println(addr, err)
        return
    }
    defer c.Close()

    e := json.NewEncoder(c)
    for m := range ch {
        err := e.Encode(m)
        if err != nil {
            log.Println(addr, err)
            return
        }
    }
}

var seenIDs = struct {
    m map[string]bool
    sync.Mutex
}{m: make(map[string]bool)}

// Seen returns true if the specified id has been seen before.
// If not, it returns false and marks the given id as "seen".
func Seen(id string) bool {
    seenIDs.Lock()
    ok := seenIDs.m[id]
    seenIDs.m[id] = true
    seenIDs.Unlock()
    return ok
}

func DiscoveryClient() {
    BROADCAST_IPv4 := net.IPv4(255, 255, 255, 255)
    socket, err := net.DialUDP("udp4", nil, &net.UDPAddr{
        IP:   BROADCAST_IPv4,
        Port: discPort,
    })
    if err != nil {
        log.Fatalf("Couldn't send UDP?!?! %v", err)
    }
    socket.Write([]byte(Self))
    log.Printf("Sent a discovery packet!")
}

func DiscoveryListen() {
    socket, err := net.ListenUDP("udp4", &net.UDPAddr{
        IP:   net.IPv4(0, 0, 0, 0),
        Port: discPort,
    })

    if err != nil {
        if e2, ok := err.(*net.OpError); ok && e2.Err.Error() == "address already in use" {
            log.Printf("UDP discovery port %d already in use. Inbound discovery disabled.", discPort)
            return
        } else {
            log.Printf("Couldn't open UDP?!? %v", err)
            log.Println("Discovery will not be possible")
            return
        }
    }
    for {

        data := make([]byte, 0)
        _, _, err := socket.ReadFromUDP(data)
        if err != nil {
            log.Fatal("Problem reading UDP packet: %v", err)
        }
        bcastAddr := string(data)
        if bcastAddr != "" && bcastAddr != Self {
            log.Printf("Adding this address to Peer List: %v", bcastAddr)
            Dial(bcastAddr)
        }

    }
}
