echo Starting to generate TLS certificates for-redis
cd ..

# if tls-gen dir exist remove it
if [ -d "tls-gen" ]; then
  rm -rf tls-gen
fi

git clone https://github.com/michaelklishin/tls-gen tls-gen
cd tls-gen/basic
make PASSWORD=
make verify
make info

# if ../../ssl/redis/certs dir not exist create it
if [ ! -d "../../ssl/redis/certs" ]; then
  mkdir ../../ssl/redis/certs
fi
mv ./result/* ../../ssl/redis/certs
echo Finished generating TLS certificates