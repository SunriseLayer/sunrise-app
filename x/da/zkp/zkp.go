package zkp

import (
	"fmt"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// Circuit defines our ZKP circuit
type Circuit struct {
	Indices   []frontend.Variable `gnark:",public"`  // indices
	Shares    []frontend.Variable `gnark:",private"` // H(s_i) values
	HShares   []frontend.Variable `gnark:",public"`  // H^2(s_i) values
	Threshold frontend.Variable   `gnark:",public"`  // threshold
}

func (circuit *Circuit) Define(api frontend.API) error {
	mimcHash, err := mimc.NewMiMC(api)
	if err != nil {
		return fmt.Errorf("failed to create MiMC hash: %v", err)
	}

	// Verify hashes of shares
	for i, j := range circuit.Indices {
		mimcHash.Write(circuit.Shares[i])
		h := mimcHash.Sum()
		j, _ := j.(int)
		api.AssertIsEqual(h, circuit.HShares[j])
	}

	// Verify that the number of shares used is at least the threshold
	api.AssertIsLessOrEqual(circuit.Threshold, frontend.Variable(len(circuit.Shares)))

	return nil
}

func ProveAndVerify(circuit Circuit) error {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), nil, &circuit) // TODO: nil
	if err != nil {
		return fmt.Errorf("compile error: %v", err)
	}

	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		return fmt.Errorf("setup error: %v", err)
	}

	witness, err := frontend.NewWitness(&circuit, ecc.BN254.ScalarField())
	if err != nil {
		return fmt.Errorf("witness creation error: %v", err)
	}

	proof, err := groth16.Prove(ccs, pk, witness)
	if err != nil {
		return fmt.Errorf("proving error: %v", err)
	}

	publicWitness, err := witness.Public()
	if err != nil {
		return fmt.Errorf("public witness error: %v", err)
	}

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		return fmt.Errorf("verification failed: %v", err)
	}

	return nil
}
