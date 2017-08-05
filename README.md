# coscup2017_grpc_golang
COSCUP 2017 golang gRPC speech to text demo server code 

###To build

You need to install [Go](https://golang.org/) to build it.

###To run
1. Get [gstreamer tools](https://gstreamer.freedesktop.org/ ) installed on your system.

2. Setup [Google Cloud Platform Speech API](https://cloud.google.com/docs/authentication/getting-started#creating_the_service_account), generate authenticate json config file.

3. Run as following, replace the *[gcp_api_auth_json_config_file_path]* with the file got from previous step :  
```
gst-launch-1.0 -v alsasr! audioconvert ! audioresample ! audio/x-raw,channels=1,rate=16000 ! filesink location=/dev/stdout | env GOOGLE_APPLICATION_CREDENTIALS=[ gcp_api_auth_json_config_file_path] ./coscup2017_grpc_golang

```

