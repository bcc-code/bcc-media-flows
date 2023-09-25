package vidispine

import (
	"os"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
)

func GetClient() *vidispine.VidispineService {

	vsapiClient := vsapi.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))

	return vidispine.NewVidispineService(vsapiClient)
}
