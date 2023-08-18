package vidispine

import (
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"os"
)

func getClient() *vidispine.Client {
	return vidispine.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))
}
