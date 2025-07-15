package main

/*func main() {
	gprcConn, err := grpc.Dial(
		"127.0.0.1:8082",
		grpc.WithInsecure(),
	)
	if err != nil {
		log.Fatalln("Cant connect to grpc")
	} else {
		fmt.Println("Connected to grpc")
	}
	defer gprcConn.Close()
	bizManager := NewBizClient(gprcConn)

	bizManager.Check(context.Background(), &Nothing{})
	bizManager.Add(context.Background(), &Nothing{})
	bizManager.Test(context.Background(), &Nothing{})
}*/
