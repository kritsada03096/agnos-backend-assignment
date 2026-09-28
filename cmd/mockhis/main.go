// mockhis is a local-only fixture server, not part of the production HIS contract.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{hospital}/patient/search/{id}", func(w http.ResponseWriter, r *http.Request) {
		hospital, id := r.PathValue("hospital"), r.PathValue("id")
		if (hospital != "a" && hospital != "b") || (id != "1111111111111" && id != "DEMO-PASSPORT") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"patient_hn": "HN-" + hospital + "-001", "national_id": "1111111111111", "passport_id": "DEMO-PASSPORT",
			"first_name_th": "ผู้ป่วยทดสอบ", "middle_name_th": nil, "last_name_th": "โรงพยาบาล " + hospital,
			"first_name_en": "Demo", "middle_name_en": nil, "last_name_en": "Hospital " + hospital,
			"date_of_birth": "1990-01-01", "phone_number": "0800000000", "email": "demo-" + hospital + "@example.test", "gender": "M",
		})
	})
	address := os.Getenv("MOCK_HIS_ADDR")
	if address == "" {
		address = ":8090"
	}
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}
