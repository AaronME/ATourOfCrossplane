package pkg

import (
	"context"
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/yaml"
)

func TestPlatformRefAWS(t *testing.T) {
	// We need to declare the GVR for both our XRs and Claims
	// This is the XR GroupVersionResource
	compositeResource := schema.GroupVersionResource{
		Group:    "aws.platformref.crossplane.io",
		Version:  "v1alpha1",
		Resource: "compositenetworks", // remember to use the plural
	}

	// This is the Claim GroupVersionResource
	claimResource := schema.GroupVersionResource{
		Group:    "aws.platformref.crossplane.io",
		Version:  "v1alpha1",
		Resource: "networks", // remember to use the plural
	}

	// This is a claim from https://github.com/upbound/platform-ref-aws/blob/main/examples/network.yaml
	claim := `---
apiVersion: aws.platformref.crossplane.io/v1alpha1
kind: Network
metadata:
  name: network
spec:
  id: platform-ref-aws-network
  clusterRef:
    id: platform-ref-aws-cluster
`

	// Unmarshal the Yaml into an Unstructured Resource.
	// This requires going through Json.
	unstructuredClaim := unstructured.Unstructured{}
	json, err := yaml.YAMLToJSON([]byte(claim))
	if err != nil {
		t.Fatal(err)
	}
	err = unstructuredClaim.UnmarshalJSON(json)
	if err != nil {
		t.Fatal(err)
	}

	// We need to capture the namespace key name here because the test name
	// changes inside Features and Assess methods
	namespaceKey := nsKey(t)

	f := features.New("Rendered").
		Assess("Managed Resources", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// We will name our claim after the namespace for the parent test.
			ns := fmt.Sprint(ctx.Value(namespaceKey))
			claimName := fmt.Sprintf("%s-claim", ns)

			unstructuredClaim.SetName(claimName)
			unstructuredClaim.SetNamespace(ns)

			// The e2e-skeleton comes with a client based on their klient type.
			// We want to use a dynamic client, which enables working with
			// unstructured resources
			dynClient, err := newDynamicClient()
			if err != nil {
				t.Fatal(err)
			}

			// Create the Claim
			createClaim, err := dynClient.Resource(claimResource).Namespace(ns).Create(context.TODO(), &unstructuredClaim, v1.CreateOptions{})
			if err != nil {
				t.Logf("creating Claim failed: +%v", createClaim)
				t.Fatal(err)
			}

			time.Sleep(3 * time.Second)

			// Retrieve the claim. This confirms our resource created and is also
			// necessary to lookup the XR name.
			getClaim, err := dynClient.Resource(claimResource).Namespace(ns).Get(context.TODO(), claimName, v1.GetOptions{})
			if err != nil {
				t.Logf("getting Claim failed: +%v", getClaim)
				t.Fatal(err)
			}

			// Get the XR
			compositeName, exists, err := unstructured.NestedString(getClaim.UnstructuredContent(), "spec", "resourceRef", "name")
			if err != nil {
				t.Fatal(err)
			}

			if exists != true {
				t.Log(getClaim.UnstructuredContent())
				t.Fatal("No composite name found.")
			}

			// Our first test: confirm that an XR was created.
			// This failure will indicate whether a successful composition template
			// was selected or not
			t.Run("Did create XR", func(t *testing.T) {
				t.Logf("Fetching XR %s", compositeName)
				getXR, err := dynClient.Resource(compositeResource).Get(context.TODO(), compositeName, v1.GetOptions{})
				if err != nil {
					t.Logf("getting XR failed: +%v", getXR)
					t.Fatal(err)
				}
			})

			// MRs we expect to be created by a Claim.
			// For the purpose of this demo, we only verify we have the correct number
			// of each resource type.
			var mrs = []struct {
				name  string
				gvr   schema.GroupVersionResource
				count int
			}{
				{"VPC", schema.GroupVersionResource{Group: "ec2.aws.crossplane.io", Version: "v1beta1", Resource: "vpcs"}, 1},
				{"InternetGateway", schema.GroupVersionResource{Group: "ec2.aws.crossplane.io", Version: "v1beta1", Resource: "internetgateways"}, 1},
				{"SecurityGroup", schema.GroupVersionResource{Group: "ec2.aws.crossplane.io", Version: "v1beta1", Resource: "securitygroups"}, 1},
				{"Subnet", schema.GroupVersionResource{Group: "ec2.aws.crossplane.io", Version: "v1beta1", Resource: "subnets"}, 4},
				{"RouteTable", schema.GroupVersionResource{Group: "ec2.aws.crossplane.io", Version: "v1beta1", Resource: "routetables"}, 1},
			}

			// MR Test Case Runs
			for _, mr := range mrs {
				mr := mr // rebind mr into this lexical scope
				t.Run(mr.name, func(t *testing.T) {

					got, err := dynClient.Resource(mr.gvr).List(context.TODO(), v1.ListOptions{})
					if err != nil {
						t.Errorf("error retrieving %q: %q", mr.name, err)
					}

					count := len(got.Items)

					if count != mr.count {
						t.Errorf("resource %q count is wrong.", mr.name)
					}
				})

			}

			return ctx
		})
	testenv.Test(t, f.Feature())
}
