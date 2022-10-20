git clone https://github.com/oracle/docker-images
cd docker-images && git config core.sparseCheckout true && git sparse-checkout init --cone && git sparse-checkout set OracleDatabase/SingleInstance
cd /OracleDatabase/SingleInstance/dockerfiles/19.3.0
https://download.oracle.com/otn/linux/oracle19c/190000/LINUX.X64_193000_db_home.zip?AuthParam=1666239845_4db0b302c703b19f30aa02bd26b3d9ee
./buildContainerImage.sh -v 19.3.0 -s -t oracle:1.19.3
docker run -it --rm --name oracledb -p 1521:1521 -p 5500:5500 -e ORACLE_PWD=My1passw -v ${PWD}:/opt/oracle/oradata oracle/database:19.3.0-se2
