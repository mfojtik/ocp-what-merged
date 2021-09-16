package release

type ImageStreamTag struct {
	Annotations map[string]string `json:"annotations"`
}

type ImageStreamSpec struct {
	Tags []ImageStreamTag `json:"tags"`
}

type ImageStream struct {
	Spec ImageStreamSpec `json:"spec"`
}
