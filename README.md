# 4-in-a-row
**Online in-browser multiplayer 4 in a row. Backend written in Go. Frontend in JavaScript.**  
Uses websockets.

**[Try it here.](https://connect4.n3wt.uk/)** (might not always be up.)

Can play with  2-6 players, though any more than 2 usually ends in a draw. Grid size customisation could fix this, but I haven't added it.

**This is designed to work behind a reverse proxy serving https, if you want to use it standalone or over http then you must change `wss://` to `ws://` in static/js/c4.js**

If used behind a reverse proxy you will get websocket errors unless you add the correct headers for location /ws.
Here is my config for nginx:

    location /ws {
                  proxy_set_header X-Real-IP $remote_addr;
                  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
                  proxy_set_header X-Forwarded-Proto $scheme;
                  proxy_set_header Host $host;
                  proxy_set_header X-NginX-Proxy true;

                  proxy_pass http://<ENTER HOST HERE>:8292;
                  proxy_redirect off;

                  proxy_http_version 1.1;
                  proxy_set_header Upgrade $http_upgrade;
                  proxy_set_header Connection "upgrade";
        }
Should be easy for apache also. 

## Docker
To install:

    docker run \
    -d \
    --name connect4 \
    -p 8292:8292 \
    --restart unless-stopped \
    en3wton/connect4
Port 8292 in the container must be published.
