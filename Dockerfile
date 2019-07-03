FROM ubuntu:latest

WORKDIR /connect4/

COPY html html
COPY static static
COPY connect4 .

EXPOSE 8292

CMD /connect4/connect4