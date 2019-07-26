import pandas as pd
import numpy as np
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib.backends.backend_pdf import PdfPages
import sys


if len(sys.argv) != 2:
	print("usage: plots.py year\n")
	1/0

year = int(sys.argv[1])

if year == 2000:
    nullCBSA = "9999"
elif year == 2010:
    nullCBSA = "99999"

def do_all(region, siz):

    if region == "cousub":
        # There is no population buffer for county subdivision
        if siz != 25000:
            return
        df = pd.read_csv("segregation_%s_%4d_norm.csv.gz" % (region, year), dtype={"CBSA": str})
        title = "%4d %ss" % (year, region)
    else:
        df = pd.read_csv("segregation_%s_%4d_%d_norm.csv.gz" % (region, year, siz), dtype={"CBSA": str})
        title = "%4d %ss, %d person buffers" % (year, region, siz)

    df = df.loc[df.RegionPop > 0, :]

    for jx in 0, 1:

        if jx == 0:
            if region == "cousub":
                continue
            # CBSA based statistics
            dm = df.loc[df.CBSA != nullCBSA, :]
            xn = "CBSA"
            tpn = "CBSATotalPop"
        elif jx == 1:
            if region != "cousub":
                continue
            # Pseudo-CBSA based statistics
            dm = df.loc[df.CBSA == nullCBSA, :]
            xn = "pseudo-CBSA"
            tpn = "PCBSATotalPop"

        plt.clf()
        plt.title(title)
        plt.hist(np.log(1+dm[tpn]))
        plt.xlabel("log outer region population based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(np.log(1 + dm.RegionPop), bins=100)
        plt.xlabel("log inner region population based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.grid(True)
        plt.plot(np.log(1+dm[tpn]), np.log(1 +dm.RegionPop), 'o', rasterized=True, alpha=0.7)
        plt.xlabel("log outer region population based on %s" % xn)
        plt.ylabel("log inner region population based on %s" % xn)
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.Neighbors, bins=np.arange(0, 60))
        plt.xlabel("Number of %ss per inner region based on %s" % (region, xn))
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.RegionRadius, bins=100)
        plt.xlabel("Inner region radius (miles) based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.LocalEntropy.dropna(), bins=100)
        plt.xlabel("Local entropy based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.RegionalEntropy.dropna(), bins=100)
        plt.xlabel("Regional entropy based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

    v = ["CBSATotalPop", "PCBSATotalPop"]
    v += ["BODissimilarity", "BODissimilarityResid"]
    v += ["WODissimilarity", "WODissimilarityResid"]
    v += ["BlackIsolation", "BlackIsolationResid"]
    v += ["WhiteIsolation", "WhiteIsolationResid"]

    for jx in 0, 1:

        if jx == 0:
            # CBSA based statistics
            if region == "cousub":
                continue
            dm = df.loc[df.CBSA != nullCBSA, v]
            xn = "CBSA"
            tpn = "CBSATotalPop"
        elif jx == 1:
            # Pseudo-CBSA based statistics
            if region != "cousub":
                continue
            dm = df.loc[df.CBSA == nullCBSA, v]
            xn = "pseudo-CBSA"
            tpn = "PCBSATotalPop"

        plt.clf()
        plt.title(title)
        plt.hist(dm.BlackIsolation.dropna(), bins=100)
        plt.xlabel("Black isolation based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.BlackIsolationResid.dropna(), bins=100)
        plt.xlabel("Adjusted black isolation based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.WhiteIsolation.dropna(), bins=100)
        plt.xlabel("White isolation based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.WhiteIsolationResid.dropna(), bins=100)
        plt.xlabel("Adjusted white isolation based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.BODissimilarity.dropna(), bins=100)
        plt.xlabel("Black/others dissimilarity based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.BODissimilarityResid.dropna(), bins=100)
        plt.xlabel("Adjusted black/others dissimilarity based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        with pd.option_context('mode.use_inf_as_null', True):
            plt.hist(dm.WODissimilarity.dropna(), bins=100)
        plt.xlabel("White/others dissimilarity based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        with pd.option_context('mode.use_inf_as_null', True):
            plt.hist(dm.WODissimilarityResid.dropna(), bins=100)
        plt.xlabel("Adjusted white/others dissimilarity based on %s" % xn)
        plt.ylabel("Frequency")
        pdf.savefig()

    va = ["CBSATotalPop", "PCBSATotalPop", "RegionPop"]
    u = [
            ["BODissimilarity", "BODissimilarityResid"],
            ["WODissimilarity", "WODissimilarityResid"],
            ["BlackIsolation", "BlackIsolationResid"],
            ["WhiteIsolation", "WhiteIsolationResid"],
        ]

    for ux in u:
        for jx in 0, 1:
            vx = va + ux
            if jx == 0:
                # CBSA based statistics
                if region == "cousub":
                    continue
                dm = df.loc[df.CBSA != nullCBSA, vx]
                xn = "CBSA"
                tpn = "CBSATotalPop"
            elif jx == 1:
                # Pseudo-CBSA based statistics
                if region != "cousub":
                    continue
                dm = df.loc[df.CBSA == nullCBSA, vx]
                xn = "pseudo-CBSA"
                tpn = "PCBSATotalPop"

            for k in 0,1:
                plt.clf()
                plt.axes([0.18, 0.1, 0.75, 0.8])
                plt.title(title)
                plt.grid(True)
                plt.plot(np.log(1 + dm[tpn]), dm[ux[k]], 'o', alpha=0.7, rasterized=True)
                plt.xlabel("log outer region population based on %s" % xn)
                plt.ylabel("%s baesd on %s" % (ux[k], xn))
                pdf.savefig()

                if region == "cousub":
                    # Inner region size only varies for cousubs
                    plt.clf()
                    plt.axes([0.18, 0.1, 0.75, 0.8])
                    plt.title(title)
                    plt.grid(True)
                    plt.plot(np.log(1 + dm.RegionPop), dm[ux[k]], 'o', alpha=0.7, rasterized=True)
                    plt.xlabel("log inner region population based on %s" % xn)
                    plt.ylabel("%s based on %s" % (ux[k], xn))
                    pdf.savefig()

            plt.clf()
            plt.axes([0.18, 0.1, 0.75, 0.8])
            plt.title(title)
            plt.grid(True)
            plt.plot(dm[ux[0]], dm[ux[1]], 'o', rasterized=True, alpha=0.7)
            plt.xlabel("%s based on %s" % (ux[0], xn))
            plt.ylabel("%s based on %s" % (ux[1], xn))
            pdf.savefig()

pdf = PdfPages("segregation_%4d.pdf" % year)

for region in "cousub", "tract", "blockgroup":
    for siz in 25000, 45000, 65000:

        #DEBUG
        if siz != 25000:
            continue

        do_all(region, siz)

pdf.close()
