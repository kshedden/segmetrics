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

def do_all(region, siz):

    if region == "cousub":
        # There is no population buffer for county subdivision
        if siz != 25000:
            return
        df = pd.read_csv("segregation_%s_%4d_norm.csv.gz" % (region, year))
        title = "%4d %ss" % (year, region)
    else:
        df = pd.read_csv("segregation_%s_%4d_%d_norm.csv.gz" % (region, year, siz))
        title = "%4d %ss, %d person buffers" % (year, region, siz)

        for v in [["CBSATotalPop", "PCBSATotalPop", "BODissimilarity", "BODissimilarityResid"],
                  ["CBSATotalPop", "PCBSATotalPop", "WODissimilarity", "WODissimilarityResid"],
                  ["CBSATotalPop", "PCBSATotalPop", "BlackIsolation", "BlackIsolationResid"],
                  ["CBSATotalPop", "PCBSATotalPop", "WhiteIsolation", "WhiteIsolationResid"]]:

            ii = df.CBSA != 99999
            dm = df.loc[ii, v]

            plt.clf()
            plt.title(title)
            plt.plot(np.log(1+dm[v[0]]), dm[v[2]], 'o', alpha=0.6, rasterized=True)
            plt.xlabel("log CBSA total population", size=15)
            plt.ylabel(v[2], size=15)
            pdf.savefig()

            plt.clf()
            plt.title(title)
            plt.plot(np.log(1+dm[v[0]]), dm[v[3]], 'o', alpha=0.6, rasterized=True)
            plt.xlabel("log CBSA total population", size=15)
            plt.ylabel(v[3], size=15)
            pdf.savefig()

            if region == "cousub":
                ii = df.CBSA == 99999
                dm = df.loc[ii, v]

                plt.clf()
                plt.title(title)
                plt.plot(np.log(1+dm[v[1]]), dm[v[2]], 'o', alpha=0.6, rasterized=True)
                plt.xlabel("log pseudo-CBSA total population", size=15)
                plt.ylabel(v[2], size=15)
                pdf.savefig()

                plt.clf()
                plt.title(title)
                plt.plot(np.log(1+dm[v[1]]), dm[v[3]], 'o', alpha=0.6, rasterized=True)
                plt.xlabel("log pseudo-CBSA total population", size=15)
                plt.ylabel(v[3], size=15)
                pdf.savefig()

    plt.clf()
    plt.title(title)
    if region == "cousub":
        plt.hist(np.log(1+df.RegionPop), bins=100)
        plt.xlabel("Log inner region population")
    else:
        plt.hist(df.RegionPop, bins=100)
        plt.xlabel("Inner region population")
    plt.ylabel("Frequency")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(df.Neighbors, bins=np.arange(0, 60))
    plt.xlabel("Number of %ss per inner region" % region)
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(df.RegionRadius, bins=100)
    plt.xlabel("Inner region radius (miles)")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(df.LocalEntropy.dropna(), bins=100)
    plt.xlabel("Local entropy")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(df.RegionalEntropy.dropna(), bins=100)
    plt.xlabel("Regional entropy")
    pdf.savefig()

    # CBSA-based statistics
    dm = df.loc[df.CBSA != 99999, :]

    plt.clf()
    plt.title(title)
    plt.hist(dm.BlackIsolation.dropna(), bins=100)
    plt.xlabel("Black isolation (CBSA-based)")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(dm.BlackIsolationResid.dropna(), bins=100)
    plt.xlabel("Adjusted black isolation (CBSA-based)")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(dm.WhiteIsolation.dropna(), bins=100)
    plt.xlabel("White isolation (CBSA-based)")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(dm.WhiteIsolationResid.dropna(), bins=100)
    plt.xlabel("Adjusted white isolation (CBSA-based)")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(dm.BODissimilarity.dropna(), bins=100)
    plt.xlabel("Black/others dissimilarity (CBSA-based)")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(dm.BODissimilarityResid.dropna(), bins=100)
    plt.xlabel("Adjusted black/others dissimilarity (CBSA-based)")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    with pd.option_context('mode.use_inf_as_null', True):
        plt.hist(dm.WODissimilarity.dropna(), bins=100)
    plt.xlabel("White/others dissimilarity (CBSA-based)")
    pdf.savefig()

    plt.clf()
    plt.title(title)
    plt.hist(dm.WODissimilarityResid.dropna(), bins=100)
    plt.xlabel("Adjusted white/others dissimilarity (CBSA-based)")
    pdf.savefig()

    if region == "cousub":

        # CBSA-based statistics
        dm = df.loc[df.CBSA == 99999, :]

        plt.clf()
        plt.title(title)
        plt.hist(np.log(1 + dm.PCBSATotalPop).dropna(), bins=100)
        plt.xlabel("log pseudo-CBSA total population (non-CBSA regions)")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.BlackIsolation.dropna(), bins=100)
        plt.xlabel("Black isolation (PCBSA-based)")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.BlackIsolationResid.dropna(), bins=100)
        plt.xlabel("Adjusted black isolation (PCBSA-based)")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.WhiteIsolation.dropna(), bins=100)
        plt.xlabel("White isolation (PCBSA-based)")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.WhiteIsolationResid.dropna(), bins=100)
        plt.xlabel("Adjusted white isolation (PCBSA-based)")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.BODissimilarity.dropna(), bins=100)
        plt.xlabel("Black/others dissimilarity (PCBSA-based)")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.BODissimilarityResid.dropna(), bins=100)
        plt.xlabel("Adjusted black/others dissimilarity (PCBSA-based)")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        with pd.option_context('mode.use_inf_as_null', True):
            plt.hist(dm.WODissimilarity.dropna(), bins=100)
        plt.xlabel("White/others dissimilarity (PCBSA-based)")
        pdf.savefig()

        plt.clf()
        plt.title(title)
        plt.hist(dm.WODissimilarityResid.dropna(), bins=100)
        plt.xlabel("Adjusted white/others dissimilarity (PCBSA-based)")
        pdf.savefig()


pdf = PdfPages("segregation_%4d.pdf" % year)

for region in "cousub", "tract", "blockgroup":
    for siz in 25000, 45000, 65000:
        do_all(region, siz)

pdf.close()
