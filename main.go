package main

import (
	"code.google.com/p/go.net/websocket"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

type Packet struct {
	X  float64
	Y  float64
	ID string
}

var (
	streams []chan<- *Packet
	lock    sync.RWMutex
	nextID  uint64
)

func Websocket(ws *websocket.Conn) {
	incoming := make(chan *Packet)

	// Max sendQ of 16
	outgoing := make(chan *Packet, 16)

	lock.Lock()
	streams = append(streams, outgoing)
	hash := md5.New()
	fmt.Fprintf(hash, "%x", nextID)
	nextID++
	id := fmt.Sprintf("%x", hash.Sum(nil))
	lock.Unlock()

	go func() {
		for {
			packet := new(Packet)
			websocket.JSON.Receive(ws, packet)
			if packet.ID == "" {
				lock.Lock()
				for i, stream := range streams {
					if stream == outgoing {
						streams = append(streams[:i], streams[i+1:]...)
						break
					}
				}
				for _, stream := range streams {
					stream <- &Packet{
						X:  -1,
						Y:  -1,
						ID: id,
					}
				}
				lock.Unlock()

				return
			}
			incoming <- packet
		}
	}()

	for {
		select {
		case p := <-incoming:
			p.ID = id
			if p.X < 0 && p.X != -1 {
				p.X = 0
			}
			if p.Y < 0 && p.Y != -1 {
				p.Y = 0
			}
			if p.X > 100 {
				p.X = 100
			}
			if p.Y > 100 {
				p.Y = 100
			}
			lock.RLock()
			for _, out := range streams {
				// Non-blocking
				select {
				case out <- p:
				default:
				}
			}
			lock.RUnlock()
		case p := <-outgoing:
			websocket.JSON.Send(ws, p)
		}
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, `<!DOCTYPE html>
<html><head>
<title>some crappy game</title>
<style>
img { -webkit-transition: 1s; -moz-transition: 1s; -o-transition: 1s; transition: 1s; }
html, body { overflow: hidden; }
</style>
</head><body>
<script>

var ws = new WebSocket( 'ws://nightgunner5.is-a-geek.net:9001/ws' );

ws.addEventListener( 'message', function( e ) {
	var p = JSON.parse( e.data );

	var el = document.querySelector( '#p' + p.ID );
	if ( el == null ) {
		el = document.createElement( 'img' );
		el.src = 'http://www.gravatar.com/avatar/' + p.ID + '?s=32&d=retro&f=y';

		el.style.marginLeft = el.style.marginTop = '-16px';

		el.id = 'p' + p.ID;
		el.style.position = 'absolute';
		document.body.appendChild( el );
	}

	if ( p.X == -1 && p.Y == -1 ) {
		el.style.opacity = 0;
		setTimeout( function() {
			document.body.removeChild( el );
		}, 1000 );
	} else {
		el.style.left = p.X + '%';
		el.style.top = p.Y + '%';
	}
}, false );

ws.addEventListener( 'open', function() {
	var delay = 0;	

	addEventListener( 'mousemove', function( e ) {
		if ( delay ) {
			e.preventDefault();
			return;
		}
		delay = setTimeout( function() { delay = 0; }, 100 );

		ws.send( JSON.stringify( {
			X: e.clientX / innerWidth * 100,
			Y: e.clientY / innerHeight * 100,
			ID: 'send'
		} ) );
		e.preventDefault();
	}, false );

	addEventListener( 'touchmove', function( e ) {
		if ( delay ) {
			e.preventDefault();
			return;
		}
		delay = setTimeout( function() { delay = 0; }, 100 );

		ws.send( JSON.stringify( {
			X: e.touches[0].clientX / innerWidth * 100,
			Y: e.touches[0].clientY / innerHeight * 100,
			ID: 'send'
		} ) );
		e.preventDefault();
	}, false );
}, false );

</script>
</body></html>`)
	})
	http.Handle("/ws", websocket.Handler(Websocket))
	log.Fatal(http.ListenAndServe(":9001", nil))
}
