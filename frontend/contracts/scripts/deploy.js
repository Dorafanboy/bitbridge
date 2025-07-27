const hre = require("hardhat");

async function main() {
  console.log("Deploying UTXO-EVM Gateway contracts...");

  // Get the deployer account
  const [deployer] = await hre.ethers.getSigners();
  console.log("Deploying with account:", deployer.address);
  console.log("Account balance:", (await deployer.getBalance()).toString());

  // Deploy UTXORegistry
  console.log("\nDeploying UTXORegistry...");
  const UTXORegistry = await hre.ethers.getContractFactory("UTXORegistry");
  const registry = await UTXORegistry.deploy();
  await registry.deployed();
  
  console.log("UTXORegistry deployed to:", registry.address);

  // Save deployment info
  const deploymentInfo = {
    network: hre.network.name,
    deployer: deployer.address,
    contracts: {
      UTXORegistry: registry.address
    },
    timestamp: new Date().toISOString()
  };

  console.log("\nDeployment Summary:");
  console.log("===================");
  console.log(`Network: ${deploymentInfo.network}`);
  console.log(`Deployer: ${deploymentInfo.deployer}`);
  console.log(`UTXORegistry: ${deploymentInfo.contracts.UTXORegistry}`);

  // Verify contracts if on a public network
  if (hre.network.name !== "localhost" && hre.network.name !== "hardhat") {
    console.log("\nWaiting for block confirmations...");
    await registry.deployTransaction.wait(6);

    console.log("Verifying contracts...");
    try {
      await hre.run("verify:verify", {
        address: registry.address,
        constructorArguments: [],
      });
    } catch (error) {
      console.log("Verification failed:", error.message);
    }
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });