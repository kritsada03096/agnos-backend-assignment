package main
import (
 "fmt"
 "os"
 "github.com/goccy/go-yaml"
)
func main() {
 for _,path := range os.Args[1:] {
  b,err:=os.ReadFile(path); if err!=nil { panic(err) }
  var v map[string]any
  if err=yaml.Unmarshal(b,&v);err!=nil {panic(err)}
  fmt.Println(path+": YAML syntax OK")
 }
}
